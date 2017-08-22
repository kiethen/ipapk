package ipapk

import (
	"archive/zip"
	"os"
	"testing"
)

func getAppZipReader(filename string) (*zip.Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func getAndroidManifest() (*zip.File, error) {
	reader, err := getAppZipReader("testdata/helloworld.apk")
	if err != nil {
		return nil, err
	}
	var xmlFile *zip.File
	for _, f := range reader.File {
		if f.Name == "AndroidManifest.xml" {
			xmlFile = f
			break
		}
	}
	return xmlFile, nil
}

func TestParseAndroidManifest(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	manifest, err := parseAndroidManifest(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if manifest.Package != "com.example.helloworld" {
		t.Errorf("got %v want %v", manifest.Package, "com.example.helloworld")
	}
	if manifest.VersionName != "1.0" {
		t.Errorf("got %v want %v", manifest.VersionName, "1.0")
	}
	if manifest.VersionCode != "1" {
		t.Errorf("got %v want %v", manifest.VersionCode, "1")
	}
}

func TestParseApkFile(t *testing.T) {
	xmlFile, err := getAndroidManifest()
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	apk, err := parseApkFile(xmlFile)
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if apk.BundleId != "com.example.helloworld" {
		t.Errorf("got %v want %v", apk.BundleId, "com.example.helloworld")
	}
	if apk.Version != "1.0" {
		t.Errorf("got %v want %v", apk.Version, "1.0")
	}
	if apk.Build != "1" {
		t.Errorf("got %v want %v", apk.Build, "1")
	}
}

func TestParseApkIconAndLabel(t *testing.T) {
	icon, label, err := parseApkIconAndLabel("testdata/helloworld.apk")
	if err != nil {
		t.Errorf("got %v want no error", err)
	}
	if len(icon) != 2035 {
		t.Errorf("got %v want %v", len(icon), 2035)
	}
	if label != "HelloWorld" {
		t.Errorf("got %v want %v", label, "HelloWorld")
	}
}
