package choochoo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/line/line-bot-sdk-go/linebot"
)

func HandleLINEWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	events, err := bot.ParseRequest(r)
	if err != nil {
		log.Printf("Cannot parse line events: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, event := range events {
		js, _ := json.Marshal(event)
		log.Printf("%s", js)

		if event.Type == linebot.EventTypeMessage {
			switch msg := event.Message.(type) {
			case *linebot.TextMessage:
				handleTextMessageEvent(msg, event)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func handleTextMessageEvent(msg *linebot.TextMessage, event *linebot.Event) {
	qText := msg.Text
	qText = strings.Replace(qText, "台", "臺", -1) // blindly replace 台 with 臺 for now

	baseQuery := fstore.
		CollectionGroup("trainStops").
		Where("station.name", "==", qText).
		Where("depart.time", ">=", time.Now()).
		OrderBy("depart.time", firestore.Asc).
		Limit(20)

	cwQuery := baseQuery.Where("train.direction", "==", 0)
	ccwQuery := baseQuery.Where("train.direction", "==", 1)

	secFlexContainer := func(label string, stops []*TrainStop) *linebot.BoxComponent {
		return &linebot.BoxComponent{
			Type:    linebot.FlexComponentTypeBox,
			Layout:  linebot.FlexBoxLayoutTypeVertical,
			Spacing: linebot.FlexComponentSpacingTypeMd,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   label,
					Size:   linebot.FlexTextSizeTypeSm,
					Weight: linebot.FlexTextWeightTypeBold,
					Color:  "#aaaaaa",
				},
				trainStopsToVerticalBox(stops),
			},
		}
	}

	queryTrains := func(q *firestore.Query) []*TrainStop {
		iter := q.Documents(context.Background())
		defer iter.Stop()

		var stops []*TrainStop
		for {
			doc, err := iter.Next()
			if err != nil {
				break
			}

			var stop TrainStop
			if err := doc.DataTo(&stop); err != nil {
				continue
			}

			if stop.Station.ID == stop.Train.Destination.ID {
				continue
			}

			stops = append(stops, &stop)
		}

		if stops != nil && len(stops) > 5 {
			stops = stops[:5]
		}

		return stops
	}

	flexContent := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:    linebot.FlexComponentTypeBox,
			Layout:  linebot.FlexBoxLayoutTypeVertical,
			Spacing: linebot.FlexComponentSpacingTypeXxl,
			Contents: []linebot.FlexComponent{
				secFlexContainer("順行", queryTrains(&cwQuery)),
				secFlexContainer("逆行", queryTrains(&ccwQuery)),
			},
		},
	}

	replyMsg := linebot.NewFlexMessage(fmt.Sprintf("%s 的最近列車", qText), flexContent)
	bot.ReplyMessage(event.ReplyToken, replyMsg).Do()
}

func trainStopsToVerticalBox(stops []*TrainStop) *linebot.BoxComponent {
	type trainTypeMeta struct {
		label string
		color string
	}

	trainTypeMetas := map[string]trainTypeMeta{
		"1":  {label: "太魯閣", color: "#d00215"},
		"2":  {label: "普悠瑪", color: "#d00215"},
		"3":  {label: "自強", color: "#d00215"},
		"4":  {label: "莒光", color: "#fe8609"},
		"5":  {label: "復興", color: "#0000a0"},
		"6":  {label: "區間", color: "#0000a0"},
		"7":  {label: "普快", color: "#888888"},
		"10": {label: "區間快", color: "#0000a0"},
	}

	var components []linebot.FlexComponent

	for _, stop := range stops {
		comp := &linebot.BoxComponent{
			Type:    linebot.FlexComponentTypeBox,
			Layout:  linebot.FlexBoxLayoutTypeHorizontal,
			Spacing: linebot.FlexComponentSpacingTypeSm,
			Contents: []linebot.FlexComponent{
				&linebot.BoxComponent{
					Type:   linebot.FlexComponentTypeBox,
					Layout: linebot.FlexBoxLayoutTypeVertical,
					Flex:   linebot.IntPtr(3),
					Contents: []linebot.FlexComponent{
						&linebot.TextComponent{
							Type: linebot.FlexComponentTypeText,
							Text: stop.Train.ID,
							Size: linebot.FlexTextSizeTypeXxs,
						},
						&linebot.TextComponent{
							Type:  linebot.FlexComponentTypeText,
							Text:  trainTypeMetas[stop.Train.TrainType.Code].label,
							Size:  linebot.FlexTextSizeTypeSm,
							Color: trainTypeMetas[stop.Train.TrainType.Code].color,
						},
					},
				},
				&linebot.TextComponent{
					Type:    linebot.FlexComponentTypeText,
					Text:    stop.Depart.Text,
					Flex:    linebot.IntPtr(3),
					Gravity: linebot.FlexComponentGravityTypeBottom,
				},
				&linebot.BoxComponent{
					Type:   linebot.FlexComponentTypeBox,
					Layout: linebot.FlexBoxLayoutTypeVertical,
					Flex:   linebot.IntPtr(5),
					Contents: []linebot.FlexComponent{
						&linebot.FillerComponent{},
						&linebot.BoxComponent{
							Type:    linebot.FlexComponentTypeBox,
							Layout:  linebot.FlexBoxLayoutTypeBaseline,
							Spacing: linebot.FlexComponentSpacingTypeXs,
							Contents: []linebot.FlexComponent{
								&linebot.TextComponent{
									Type:  linebot.FlexComponentTypeText,
									Text:  "往",
									Flex:  linebot.IntPtr(0),
									Size:  linebot.FlexTextSizeTypeXxs,
									Color: "#9e9e9e",
								},
								&linebot.TextComponent{
									Type: linebot.FlexComponentTypeText,
									Text: stop.Train.Destination.Name,
								},
							},
						},
					},
				},
				&linebot.TextComponent{
					Type:    linebot.FlexComponentTypeText,
					Text:    "無狀態",
					Flex:    linebot.IntPtr(5),
					Size:    linebot.FlexTextSizeTypeSm,
					Gravity: linebot.FlexComponentGravityTypeBottom,
					Color:   "#9e9e9e",
				},
			},
		}

		components = append(components, comp)
	}

	if len(components) == 0 {
		components = []linebot.FlexComponent{
			&linebot.SpacerComponent{
				Type: linebot.FlexComponentTypeSpacer,
				Size: linebot.FlexSpacerSizeTypeLg,
			},
			&linebot.TextComponent{
				Type:  linebot.FlexComponentTypeText,
				Text:  "無結果",
				Size:  linebot.FlexTextSizeTypeSm,
				Align: linebot.FlexComponentAlignTypeCenter,
				Color: "#b0b0b0",
			},
			&linebot.SpacerComponent{
				Type: linebot.FlexComponentTypeSpacer,
				Size: linebot.FlexSpacerSizeTypeLg,
			},
		}
	}

	box := &linebot.BoxComponent{
		Type:     linebot.FlexComponentTypeBox,
		Layout:   linebot.FlexBoxLayoutTypeVertical,
		Spacing:  linebot.FlexComponentSpacingTypeMd,
		Contents: components,
	}

	return box
}
