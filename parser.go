package ipapk

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"image/png"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/DHowett/go-plist"
	"github.com/shogo82148/androidbinary"
	"github.com/shogo82148/androidbinary/apk"
)

var reInfoPlist = regexp.MustCompile(`Payload/[^/]+/Info\.plist`)

type PlatformType int

const (
	PlatformTypeAndroid PlatformType = 1 + iota
	PlatformTypeIOS
)

type AppInfo struct {
	Name     string
	BundleId string
	Version  string
	Build    string
	Icon     []byte
}

type androidManifest struct {
	Package     string `xml:"package,attr"`
	VersionName string `xml:"versionName,attr"`
	VersionCode string `xml:"versionCode,attr"`
}

type iosPlist struct {
	CFBundleDisplayName  string `plist:"CFBundleDisplayName"`
	CFBundleVersion      string `plist:"CFBundleVersion"`
	CFBundleShortVersion string `plist:"CFBundleShortVersionString"`
	CFBundleIdentifier   string `plist:"CFBundleIdentifier"`
}

func NewAppParser(name string, platformType PlatformType) (*AppInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}

	var xmlFile, plistFile, iosIconFile *zip.File
	for _, f := range reader.File {
		switch {
		case f.Name == "AndroidManifest.xml":
			xmlFile = f
		case reInfoPlist.MatchString(f.Name):
			plistFile = f
		case strings.Contains(f.Name, "AppIcon60x60"):
			iosIconFile = f
		}
	}

	if platformType == PlatformTypeAndroid {
		appInfo, err := parseApkFile(xmlFile)
		icon, label, err := parseApkIconAndLabel(name)
		appInfo.Name = label
		appInfo.Icon = icon
		return appInfo, err
	}

	if platformType == PlatformTypeIOS {
		appInfo, err := parseIpaFile(plistFile)
		icon, err := parseIpaIcon(iosIconFile)
		appInfo.Icon = icon
		return appInfo, err
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

func parseApkFile(xmlFile *zip.File) (*AppInfo, error) {
	if xmlFile == nil {
		return nil, errors.New("AndroidManifest.xml is not found")
	}

	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		return nil, err
	}

	appInfo := new(AppInfo)
	appInfo.BundleId = manifest.Package
	appInfo.Version = manifest.VersionName
	appInfo.Build = manifest.VersionCode

	return appInfo, nil
}

func parseApkIconAndLabel(name string) ([]byte, string, error) {
	pkg, err := apk.OpenFile(name)
	if err != nil {
		return nil, "", err
	}
	defer pkg.Close()

	icon, _ := pkg.Icon(nil)
	if icon == nil {
		return nil, "", errors.New("Icon is not found")
	}

	label, _ := pkg.Label(nil)

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, icon); err != nil {
		return nil, "", err
	}

	return buf.Bytes(), label, nil
}

func parseIpaFile(plistFile *zip.File) (*AppInfo, error) {
	if plistFile == nil {
		return nil, errors.New("info.plist is not found")
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

	info := new(iosPlist)
	decoder := plist.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(info); err != nil {
		return nil, err
	}

	appInfo := new(AppInfo)
	appInfo.Name = info.CFBundleDisplayName
	appInfo.BundleId = info.CFBundleIdentifier
	appInfo.Version = info.CFBundleShortVersion
	appInfo.Build = info.CFBundleVersion

	return appInfo, nil
}

func parseIpaIcon(iconFile *zip.File) ([]byte, error) {
	if iconFile == nil {
		return nil, errors.New("Icon is not found")
	}

	rc, err := iconFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return ioutil.ReadAll(rc)
}
