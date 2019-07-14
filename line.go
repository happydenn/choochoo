package choochoo

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
)

var (
	bot                    *linebot.Client
	lineChannelSecret      string
	lineChannelAccessToken string
)

func init() {
	var err error

	lineChannelSecret = os.Getenv("LINE_CHANNEL_SECRET")
	lineChannelAccessToken = os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	bot, err = linebot.New(lineChannelSecret, lineChannelAccessToken, linebot.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("Cannot initialize LINE client: %s", err)
	}
}
