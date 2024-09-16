package main

import (
	"context"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"log"
	"os"
)

func main() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	f, err := os.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = f.Close()
	}()

	var buf []byte
	err = chromedp.Run(ctx, chromedp.Tasks{
		// http://127.0.0.1:8080/test.md?print-pdf
		chromedp.Navigate(`https://stackoverflow.com/`),
		// chromedp.WaitVisible(`body`), // 有的頁面會需要
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err = page.PrintToPDF().Do(ctx)
			if err != nil {
				return err
			}
			_, err = f.Write(buf)
			return err
		}),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("包含超連結的PDF檔案生成成功")
}
