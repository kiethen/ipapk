package ipapk

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DHowett/go-plist"
	"github.com/andrianbdn/iospng"
	"gitlab.com/omnarayan/androidbinary"
	"gitlab.com/omnarayan/androidbinary/apk"
)

var reInfoPlist = regexp.MustCompile(`Payload/[^/]+/Info\.plist`)

const (
	iosExt     = ".ipa"
	androidExt = ".apk"
)

type appInfo struct {
	Name             string
	PackageName      string
	Version          string
	Build            string
	Base64           string
	VersionName      string
	VersionCode      string
	SDKMinVersion    int
	SDKTargetVersion int
	UsesPermissions  []string
	Activities       []string
	LanchActivity    string
	Icon             image.Image
	Size             int64
	IconFileName     string
}

type usesPermissions struct {
	Name string `xml:"name,attr"`
}
type sdkVersion struct {
	Min    int `xml:"minSdkVersion,attr"`
	Target int `xml:"targetSdkVersion,attr"`
}
type activity struct {
	Name     string   `xml:"name,attr"`
	Exported bool     `xml:"exported,attr"`
	Filter   []filter `xml:"intent-filter"`
}
type action struct {
	Name string `xml:"name,attr"`
}
type category struct {
	Name string `xml:"name,attr"`
}
type filter struct {
	Action   action   `xml:"action"`
	Category category `xml:"category"`
}
type androidManifest struct {
	Package         string            `xml:"package,attr"`
	VersionName     string            `xml:"versionName,attr"`
	VersionCode     string            `xml:"versionCode,attr"`
	SDKVersion      sdkVersion        `xml:"uses-sdk"`
	UsesPermissions []usesPermissions `xml:"uses-permission"`
	Activities      []activity        `xml:"application>activity"`
}

type iosPlist struct {
	CFBundleName         string                            `plist:"CFBundleName"`
	CFBundleDisplayName  string                            `plist:"CFBundleDisplayName"`
	CFBundleVersion      string                            `plist:"CFBundleVersion"`
	CFBundleShortVersion string                            `plist:"CFBundleShortVersionString"`
	CFBundleIdentifier   string                            `plist:"CFBundleIdentifier"`
	CFBundleExecutable   string                            `plist:"CFBundleExecutable"`
	CFBundleIcons        map[string]map[string]interface{} `plist:"CFBundleIcons"`
}

func NewAppParser(name string) (*appInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		fmt.Println(name, " Unable to find file ", err)
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		fmt.Println(name, " Unable to find file stat", err)
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		fmt.Println(" Unable to parse zip file ", err)
		return nil, err
	}

	var xmlFile, plistFile, iosIconFile *zip.File
	for _, f := range reader.File {
		switch {
		case f.Name == "AndroidManifest.xml":
			xmlFile = f
		case reInfoPlist.MatchString(f.Name):
			plistFile = f
		case strings.Contains(f.Name, "Info.plist"):
			plistFile = f
		case strings.Contains(f.Name, "AppIcon60x60"):
			iosIconFile = f
		}
	}

	ext := filepath.Ext(stat.Name())

	if ext == androidExt {
		info, err := parseApkFile(xmlFile)
		icon, label, err := parseApkIconAndLabel(name)
		if err != nil {
			icon = nil
			fmt.Println("Unable to get icon information ", err)
			err = nil

		}
		info.Name = label
		// info.Icon = icon
		info.Size = stat.Size()
		if icon != nil {
			var buff bytes.Buffer

			png.Encode(&buff, icon)
			info.Base64 = base64.StdEncoding.EncodeToString(buff.Bytes())
		}
		return info, err
	}

	if ext == iosExt || ext == ".zip" {
		info, err := parseIpaFile(plistFile)

		if err != nil {
			fmt.Println("Error in getting iOS app details ", err)
			return nil, err
		}
		for _, f := range reader.File {
			if strings.Contains(f.Name, info.IconFileName) {
				iosIconFile = f
				fmt.Println("We got icon file ", iosIconFile)
			}

		}
		icon, err := parseIpaIcon(iosIconFile)
		if err != nil {
			fmt.Println("Error in getting iOS app icon ", err)
		} else {
			info.Icon = icon
			var buff bytes.Buffer
			png.Encode(&buff, icon)
			info.Base64 = base64.StdEncoding.EncodeToString(buff.Bytes())
			info.Size = stat.Size()
		}

		return info, nil
	}

	return nil, errors.New("unknown platform")
}

func parseAndroidManifest(xmlFile *zip.File) (*androidManifest, error) {
	rc, err := xmlFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	xmlContent, err := androidbinary.NewXMLFile(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	manifest := new(androidManifest)
	decoder := xml.NewDecoder(xmlContent.Reader())
	if err := decoder.Decode(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func parseApkFile(xmlFile *zip.File) (*appInfo, error) {
	if xmlFile == nil {
		return nil, errors.New("AndroidManifest.xml is not found")
	}

	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("Activities %+v\n", manifest.Activities)
	// fmt.Println("UsesPermissions %+v\n", manifest.UsesPermissions)
	info := new(appInfo)
	info.PackageName = manifest.Package
	info.Version = manifest.VersionName
	info.Build = manifest.VersionCode
	info.SDKMinVersion = manifest.SDKVersion.Min
	info.SDKTargetVersion = manifest.SDKVersion.Target
	info.VersionCode = manifest.VersionCode
	info.VersionName = manifest.VersionName

	// LanchActivity
	for _, activity := range manifest.Activities {
		for _, filter := range activity.Filter {
			if filter.Action.Name == "android.intent.action.MAIN" && filter.Category.Name == "android.intent.category.LAUNCHER" {
				info.LanchActivity = activity.Name
				break
			}

		}
	}
	return info, nil
}

func parseApkIconAndLabel(name string) (image.Image, string, error) {
	pkg, err := apk.OpenFile(name)
	if err != nil {
		return nil, "", err
	}
	defer pkg.Close()
	icon, _ := pkg.Icon(&androidbinary.ResTableConfig{
		Density: 720,
	})
	if icon == nil {
		return nil, "", errors.New("Icon is not found")
	}

	label, _ := pkg.Label(nil)
	args := []string{"dump", "badging", name}
	resp, _ := exec.Command("aapt", args...).Output()
	if len(resp) > 0 {
		data := strings.Split(string(resp), "launchable-activity")
		if len(data) > 0 {
			temp := strings.Split(data[1], "label='")
			if len(temp) > 0 {
				temp1 := strings.Split(temp[1], "'")
				label = strings.Replace(temp1[0], "'", "", -1)
			}
		}
	}

	return icon, label, nil
}

func parseIpaFile(plistFile *zip.File) (*appInfo, error) {
	if plistFile == nil {
		return nil, errors.New("not zip file, info.plist is not found")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	p := new(iosPlist)
	decoder := plist.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}

	info := new(appInfo)
	if p.CFBundleDisplayName == "" {
		info.Name = p.CFBundleName
	} else {
		info.Name = p.CFBundleDisplayName
	}
	info.PackageName = p.CFBundleIdentifier
	info.Version = p.CFBundleShortVersion
	info.VersionCode = p.CFBundleVersion
	info.VersionName = p.CFBundleShortVersion
	info.LanchActivity = p.CFBundleExecutable
	if p.CFBundleIcons["CFBundlePrimaryIcon"] != nil {
		for _, v := range p.CFBundleIcons["CFBundlePrimaryIcon"]["CFBundleIconFiles"].([]interface{}) {
			info.IconFileName = v.(string)
		}
	}

	return info, nil
}

func parseIpaIcon(iconFile *zip.File) (image.Image, error) {
	if iconFile == nil {
		return nil, errors.New("Icon is not found")
	}

	rc, err := iconFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var w bytes.Buffer
	iospng.PngRevertOptimization(rc, &w)

	return png.Decode(bytes.NewReader(w.Bytes()))
}
