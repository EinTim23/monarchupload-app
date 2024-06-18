package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"embed"

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

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
	void HideDockIcon(int shouldHide) {
		if (shouldHide)
	    	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		else
			[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	    return;
	}
*/
import "C"

//go:embed watermark.png
var watermark []byte

//go:embed logo.png
var logo []byte
var dummy embed.FS

func hideFromDock(shouldHide bool) {
	if shouldHide {
		C.HideDockIcon(1)
	} else {
		C.HideDockIcon(0)
	}
}

type UploadResponse struct {
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func uploadFile(path string, uploadSecret string) {
	/*
		TODO:
			- add support for file chunking to support files over 100mb
			- add support for private files
			- clean up code
	*/
	client := &http.Client{}
	body := new(bytes.Buffer)

	mwriter := multipart.NewWriter(body)
	secret, _ := mwriter.CreateFormField("secret")
	secret.Write([]byte(uploadSecret))
	file, _ := mwriter.CreateFormFile("file", filepath.Base(path))
	fileHandle, err := os.Open(path)
	if err != nil {
		return
	}
	defer fileHandle.Close()
	io.Copy(file, fileHandle)
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
	resourcesPath := filepath.Join(filepath.Dir(path), "../Resources")
	beeep.Alert("MonarchUpload", response.Message, resourcesPath+"/logo.png")
	if response.Status == "success" {
		clipboard.Write(clipboard.FmtText, []byte(response.Data.URL))
	}
}
func main() {
	fs.ReadFile(dummy, "") //i need this dummy so the go static check doesnt automatically remove the embed module from my imports(thanks vscode)
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
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
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
		path, _ := zenity.SelectFile(zenity.Directory())
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
