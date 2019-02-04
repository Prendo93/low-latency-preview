package handlers

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Prendo93/low-latency-preview/utils"
	m3u8 "github.com/Prendo93/m3u8-1"
	"github.com/gorilla/mux"
	"github.com/zencoder/go-dash/mpd"
)

// UploadHandler handles for http upload
type FileUploadHandler struct {
	BaseDir string
}

func (u *FileUploadHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	utils.GetUploadLogger().Infof("Received upload request\n")
	curFileURL := req.URL.EscapedPath()[len("/ldash"):]
	vars := mux.Vars(req)
	folder := vars["folder"]
	curFolderPath := path.Join(u.BaseDir, folder)
	curFilePath := path.Join(u.BaseDir, curFileURL)
	u.serveHTTPImpl(curFolderPath, curFilePath, w, req)
}

func (u *FileUploadHandler) serveHTTPImpl(curFolderPath string, curFilePath string, w http.ResponseWriter, req *http.Request) {
	if _, err := os.Stat(curFolderPath); os.IsNotExist(err) {
		err := os.MkdirAll(curFolderPath, os.ModePerm)
		if err != nil {
			utils.GetUploadLogger().Infof("fail to create file %v", err)
		}
	}

	// rewrite, mostly for manifest file
	if _, err := os.Stat(curFilePath); err == nil {
		utils.GetUploadLogger().Debugf("rewrite file %s @ %v \n", curFilePath, time.Now().Format(time.RFC3339))
		data, _ := ioutil.ReadAll(req.Body)
		err = ioutil.WriteFile(curFilePath, data, 0644)
		if err != nil {
			utils.GetUploadLogger().Errorf("fail to create file %v \n", err)
		}
		err = createHlsManifest(curFilePath)
		if err != nil {
			utils.GetUploadLogger().Errorf("Failed to create hls manifest %v \n", err)
		}

		return
	}

	// create, mostly for segment
	// for segment, we will allow partial downloading during the uploading to save the time for player(this is what low latency meaning)
	// So here uses Symlink as a signal to tell download handler whether the uploading is finished or not.
	symlink := curFilePath + ".symlink"
	os.Symlink(curFilePath, symlink)
	utils.GetUploadLogger().Debugf("create symlink %s @ %v \n", symlink, time.Now().Format(time.RFC3339))

	f, rerr := os.Create(curFilePath)
	if rerr != nil {
		utils.GetUploadLogger().Errorf("fail to create file %s : %v\n", curFilePath, rerr)
		return
	}

	utils.GetUploadLogger().Debugf("create file %s @ %v \n", curFilePath, time.Now().Format(time.RFC3339))
	defer f.Close()

	_, rerr = io.Copy(f, req.Body)
	if rerr != nil {
		utils.GetUploadLogger().Errorf("fail to create file %v \n", rerr)
	}

	// remove symlink once the uploading is done
	os.Remove(symlink)
	utils.GetUploadLogger().Debugf("remove symlink %s @ %v \n", symlink, time.Now().Format(time.RFC3339))

}

func createHlsManifest(curFilePath string) error {
	// here we write the hls manifest
	// parse it into dash
	Mpd, err := mpd.ReadFromFile(curFilePath)
	if err != nil {
		return err
	}
	utils.GetUploadLogger().Debugf("got mpd %v \n", Mpd)
	// Create a new hls media playlist
	mPl, err := m3u8.NewMediaPlaylist(1024, 1024)
	if err != nil {
		return err
	}
	representationId := Mpd.Periods[0].AdaptationSets[0].Representations[0].ID
	st := Mpd.Periods[0].AdaptationSets[0].Representations[0].SegmentTemplate
	mPl.TargetDuration = float64(*st.Duration) / 1000000
	// Get all the segments
	filenameTemplate := *st.Media
	i := *st.StartNumber
	for {
		filename := insertIdsIntoTemplateString(filenameTemplate, *representationId, i)
		dir := filepath.Dir(curFilePath)
		if _, err := os.Stat(filepath.Join(dir, filename)); os.IsNotExist(err) {
			break
		}
		mPl.AppendSegment(&m3u8.MediaSegment{
			SeqId:    uint64(i),
			URI:      filename,
			Duration: float64(*st.Duration) / 1000000,
			Prefetch: !isFileUploadingDone(filepath.Join(dir, filename)),
		})
		i++
	}
	// Add one more prefetch segment to signal two ahead
	filename := insertIdsIntoTemplateString(filenameTemplate, *representationId, i)
	mPl.AppendSegment(&m3u8.MediaSegment{
		SeqId:    uint64(i),
		URI:      filename,
		Duration: float64(*st.Duration) / 1000000,
		Prefetch: true,
	})
	initUriString := insertIdsIntoTemplateString(*st.Initialization, *representationId, 0)
	mPl.Segments[0].Map = &m3u8.Map{
		URI: initUriString,
	}
	hlsFileName := strings.TrimSuffix(curFilePath, ".mpd") + ".m3u8"
	err = ioutil.WriteFile(hlsFileName, []byte(mPl.String()), 0644)
	if err != nil {
		return err
	}
	return nil
}

func insertIdsIntoTemplateString(filenameTemplate, representationId string, i int64) string {
	filename := strings.Replace(filenameTemplate, "$RepresentationID$", representationId, -1)
	return strings.Replace(filename, "$Number%05d$", fmt.Sprintf("%05d", i), -1)
}

func isFileUploadingDone(file string) bool {
	symlink := file + ".symlink"
	if _, err := os.Stat(symlink); err == nil {
		// exist, then segment uploading is not finished yet
		return false
	}
	// not exist
	return true
}
