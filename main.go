package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/emersion/go-autostart"
	"github.com/fsnotify/fsnotify"
	"github.com/gen2brain/beeep"
	"github.com/ncruces/zenity"
	"golang.design/x/clipboard"
)

import "C"

//go:embed watermark.png
var watermark []byte

//go:embed logo.png
var logo []byte

type UploadResponse struct {
	Data struct {
		URL string `json:"url"`
	} `json:"data,omitempty"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func isFileInUse(filename string) bool {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_EXCL, 0666)
	if err != nil {
		return true
	}
	file.Close()
	return false
}
func uploadFile(path string, uploadSecret string) {
	/*
		TODO:
			- add support for private files
			- clean up code
			- make my own toast library because all cross platform toast libraries arent great
	*/
	fileHandle, err := os.Open(path)
	if err != nil {
		return
	}
	defer fileHandle.Close()

	lastChunk := false
	var chunkSize int64 = 5000000 //50 mb
	var chunk int64 = 0
	client := &http.Client{}

	for {
		body := new(bytes.Buffer)

		mwriter := multipart.NewWriter(body)
		secret, _ := mwriter.CreateFormField("secret")
		secret.Write([]byte(uploadSecret))

		chunked, _ := mwriter.CreateFormField("chunked")
		chunked.Write([]byte("true"))

		private, _ := mwriter.CreateFormField("private")
		private.Write([]byte("false"))

		file, _ := mwriter.CreateFormFile("file", filepath.Base(path))
		reader := io.NewSectionReader(fileHandle, chunk*chunkSize, chunkSize)
		written, _ := io.Copy(file, reader)
		if written != chunkSize {
			lastChunk = true
		}
		lastchunk, _ := mwriter.CreateFormField("lastchunk")
		if lastChunk {
			lastchunk.Write([]byte("true"))
		} else {
			lastchunk.Write([]byte("false"))
		}

		mwriter.Close()

		req, err := http.NewRequest(http.MethodPost, "https://api.monarchupload.cc/v3/upload", body)
		if err != nil {
			return
		}
		req.Header.Add("Content-Type", mwriter.FormDataContentType())
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		var bodyData []byte
		bodyData, err = io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		var response UploadResponse
		err = json.Unmarshal(bodyData, &response)
		if err != nil {
			return
		}
		if response.Status != "success" || lastChunk {
			beeep.Alert("MonarchUpload", response.Message, "")
			if response.Data.URL != "" {
				clipboard.Write(clipboard.FmtText, []byte(response.Data.URL))
			}
			return
		}
		chunk++
	}
}
func main() {
	path, _ := os.Executable()
	autostartApp := &autostart.App{
		Name:        "MonarchUpload",
		DisplayName: "MonarchUpload",
		Exec:        []string{path},
	}
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	a := app.NewWithID("dev.eintim.monarchupload")
	installed := a.Preferences().Bool("installed")
	if !installed {
		a.Preferences().SetBool("installed", true)
	}

	a.SetIcon(fyne.NewStaticResource("icon", logo))
	go func() {
		lastImage := ""
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && lastImage != event.Name {
					lastImage = event.Name
					for {
						if !isFileInUse(event.Name) {
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
					uploadFile(event.Name, a.Preferences().String("uploadsecret"))
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	watcher.Add(a.Preferences().String("path"))
	w := a.NewWindow("Monarch")
	w.SetFixedSize(true)
	image := canvas.NewImageFromReader(bytes.NewReader(watermark), "watermark")
	image.SetMinSize(fyne.NewSize(430, 80))
	launchOnBootToggle := widget.NewCheck("Launch on system boot", func(value bool) {
		a.Preferences().SetBool("autostart", value)
		if value {
			autostartApp.Enable()
		} else {
			autostartApp.Disable()
		}
	})
	launchOnBootToggle.Checked = a.Preferences().Bool("autostart")
	selectFolderButton := widget.NewButton("Select folder 📂", func() {
		path, err := zenity.SelectFile(zenity.Directory())
		if err != nil {
			return
		}
		a.Preferences().SetString("path", path)
		for i := 0; i < len(watcher.WatchList()); i++ {
			watcher.Remove(watcher.WatchList()[i])
		}
		watcher.Add(path)
	})

	uploadSecretInput := widget.NewEntry()
	uploadSecretInput.SetPlaceHolder("Upload secret")
	uploadSecretInput.SetText(a.Preferences().String("uploadsecret"))
	uploadSecretInput.OnChanged = func(value string) {
		a.Preferences().SetString("uploadsecret", value)
	}
	vbox := container.NewVBox(image, launchOnBootToggle, uploadSecretInput, selectFolderButton)

	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("MonarchUpload",
			fyne.NewMenuItem("Upload file", func() {
				path, err := zenity.SelectFile()
				if err != nil {
					return
				}
				uploadFile(path, a.Preferences().String("uploadsecret"))
			}),
			fyne.NewMenuItem("Upload folder", func() {
				path, err := zenity.SelectFile(zenity.Directory())
				if err != nil {
					return
				}
				filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
					if info.IsDir() {
						return nil
					}
					uploadFile(filePath, a.Preferences().String("uploadsecret"))
					return nil
				})

			}),
			fyne.NewMenuItem("Settings", func() {
				hideFromDock(false)
				w.Show()
			}))
		desk.SetSystemTrayMenu(m)
	}
	w.SetCloseIntercept(func() {
		w.Hide()
		hideFromDock(true)
	})
	if installed {
		go func() {
			time.Sleep(time.Second)
			w.Hide()
			hideFromDock(true)
		}()
	}

	w.SetContent(vbox)
	w.ShowAndRun()
}
