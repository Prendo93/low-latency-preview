package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// RemoveContents remove the all files in the work folder at beginning
func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		GetGCloadLogger().Errorf("%v\n", err)
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			GetGCloadLogger().Errorf("%v\n", err)
			return err
		}
	}
	return nil
}

const masterString = `#EXTM3U
#EXT-X-VERSION:4
#EXT-X-STREAM-INF:PROGRAM-ID=0,BANDWIDTH=346214,CODECS="avc1.4d4015",RESOLUTION=240x426
manifest.m3u8`

func WriteMasterFile(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, "master.m3u8"), []byte(masterString), 0644)
}
