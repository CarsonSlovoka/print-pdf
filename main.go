package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := os.Stat("index.html"); os.IsNotExist(err) {
		_, _ = w.Write(indexBytes)
	} else {
		var bs []byte
		if bs, err = os.ReadFile("index.html"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(bs)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		http.Error(w,
			fmt.Sprintf(
				"The Content-Type: %s is not supported. This API only accepts: 'multipart/form-data'",
				contentType,
			),
			http.StatusUnsupportedMediaType,
		)
		return
	}

	if err := r.ParseMultipartForm(int64(1 << 20)); err != nil { // 1 MB
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetURL := r.PostForm.Get("url")
	if targetURL == "" || !strings.HasPrefix(targetURL, "http") {
		http.Error(w, "The URL does not start with http or empty", http.StatusBadRequest)
		return
	}

	result := struct {
		Status int    `json:"status"`
		Msg    string `json:"msg,omitempty"`
	}{}

	waitVisible := r.PostForm.Get("waitVisible")
	filename := r.PostForm.Get("filename")

	displayHeaderFooter := r.PostForm.Get("displayHeaderFooter") == "1"
	printBackground := r.PostForm.Get("printBackground") == "1"

	width, err1 := strconv.ParseFloat(r.PostForm.Get("width"), 64)
	height, err2 := strconv.ParseFloat(r.PostForm.Get("height"), 64)
	if errBorder := errors.Join(err1, err2); errBorder != nil {
		http.Error(w, "size error"+errBorder.Error(), http.StatusBadRequest)
		return
	}

	marginTop, err1 := strconv.ParseFloat(r.PostForm.Get("top"), 64)
	marginBottom, err2 := strconv.ParseFloat(r.PostForm.Get("bottom"), 64)
	marginLeft, err3 := strconv.ParseFloat(r.PostForm.Get("left"), 64)
	marginRight, err4 := strconv.ParseFloat(r.PostForm.Get("right"), 64)
	if errBorder := errors.Join(err1, err2, err3, err4); errBorder != nil {
		http.Error(w, "margin error"+errBorder.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	ctx2, cancel2 := chromedp.NewContext(ctx)
	defer cancel2()

	tasks := chromedp.Tasks{
		chromedp.Navigate(targetURL),
	}
	if waitVisible != "" {
		tasks = append(tasks, chromedp.WaitVisible(waitVisible))
	}

	tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()
		var buf []byte
		buf, _, err = page.PrintToPDF().
			WithDisplayHeaderFooter(displayHeaderFooter).
			WithPrintBackground(printBackground). // 建議啟用

			// a4 8.3 x 11.7
			WithPaperWidth(width).
			WithPaperHeight(height).

			// 邊界設定為0
			WithMarginTop(marginTop).
			WithMarginBottom(marginBottom).
			WithMarginLeft(marginLeft).
			WithMarginRight(marginRight).
			Do(ctx)
		if err != nil {
			return err
		}
		_, err = f.Write(buf)
		return err
	}))

	if err := chromedp.Run(ctx2, tasks); err != nil {
		result.Status = http.StatusInternalServerError
		result.Msg = err.Error()
	} else {
		result.Status = http.StatusOK
		result.Msg = fmt.Sprintf("file created: %s | %s", filename, time.Now().Format("2006/01/02 15:04:05"))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&result)
}

//go:embed index.html
var indexBytes []byte

func main() {
	var port int
	flag.IntVar(&port, "port", 9000, "port number")
	flag.Parse()

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/download", handleDownload)

	if err := http.ListenAndServe("127.0.0.1:"+strconv.Itoa(port), nil); err != nil {
		log.Fatal(err)
	}
}
