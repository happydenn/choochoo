package choochoo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/functions/metadata"
	"google.golang.org/genproto/googleapis/type/latlng"
)

type TrainStop struct {
	Train        *Train    `firestore:"train"`
	StopSequence int       `firestore:"stopSequence"`
	Station      *Station  `firestore:"station"`
	Arrive       *StopTime `firestore:"arrive"`
	Depart       *StopTime `firestore:"depart"`
	UpdateTime   time.Time `firestore:"updateTime,serverTimestamp"`
}

type Train struct {
	ID          string     `firestore:"id"`
	Destination *Station   `firestore:"destination"`
	Direction   *int       `firestore:"direction,omitempty"`
	TrainType   *TrainType `firestore:"trainType,omitempty"`
	Notes       string     `firestore:"notes,omitempty"`
}

type TrainType struct {
	ID   string `firestore:"id"`
	Name string `firestore:"name"`
	Code string `firestore:"code,omitempty"`
}

type StopTime struct {
	Time time.Time `firestore:"time"`
	Text string    `firestore:"text"`
}

type Station struct {
	ID              string         `firestore:"id"`
	Name            string         `firestore:"name"`
	ReservationCode string         `firestore:"reservationCode,omitempty"`
	Location        *latlng.LatLng `firestore:"location,omitempty"`
	Address         string         `firestore:"address,omitempty"`
	Phone           string         `firestore:"phone,omitempty"`
	StationClass    string         `firestore:"stationClass,omitempty"`
}

func UpdateTimetable(ctx context.Context, _ interface{}) error {
	m, err := metadata.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("parse metadata error: %s", err)
	}

	loc, _ := time.LoadLocation("Asia/Taipei")
	now := time.Now().In(loc)

	lag := now.Sub(m.Timestamp)
	log.Printf("trigger latency: %.0fms", float64(lag/time.Millisecond))

	var dates []string
	for i := 0; i < 7; i++ {
		s := now.Add(time.Duration(i) * (24 * time.Hour)).Format("2006-01-02")
		dates = append(dates, s)
	}

	log.Printf("Updating timetables for dates: %v", dates)

	fetch := func(dstr string) error {
		log.Printf("Updating %s", dstr)

		q := &url.Values{}
		q.Set("$count", "true")

		r, err := ptxClient.Get(fmt.Sprintf("/MOTC/v3/Rail/TRA/DailyTrainTimetable/TrainDate/%s", dstr), q)
		if err != nil {
			return fmt.Errorf("ptx api error: %s", err)
		}

		var result struct {
			Count           int    `json:"Count"`
			TrainDate       string `json:"TrainDate"`
			UpdateTime      string `json:"UpdateTime"`
			TrainTimetables []struct {
				TrainInfo struct {
					TrainNo       string `json:"TrainNo"`
					Direction     int    `json:"Direction"`
					TrainTypeID   string `json:"TrainTypeID"`
					TrainTypeCode string `json:"TrainTypeCode"`
					TrainTypeName struct {
						ZhTW string `json:"Zh_tw"`
					} `json:"TrainTypeName"`
					StartingStationID   string `json:"StartingStationID"`
					StartingStationName struct {
						ZhTW string `json:"Zh_tw"`
					} `json:"StartingStationName"`
					EndingStationID   string `json:"EndingStationID"`
					EndingStationName struct {
						ZhTW string `json:"Zh_tw"`
					} `json:"EndingStationName"`
					Note string `json:"Note"`
				} `json:"TrainInfo"`
				StopTimes []struct {
					StopSequence int    `json:"StopSequence"`
					StationID    string `json:"StationID"`
					StationName  struct {
						ZhTW string `json:"Zh_tw"`
					} `json:"StationName"`
					ArrivalTime   string `json:"ArrivalTime"`
					DepartureTime string `json:"DepartureTime"`
				} `json:"StopTimes"`
			} `json:"TrainTimetables"`
		}

		if err := json.Unmarshal(r.Data(), &result); err != nil {
			return fmt.Errorf("cannot unmarshal response: %s", err)
		}

		if result.Count != len(result.TrainTimetables) {
			return fmt.Errorf("received incomplete data. total: %v, expect %v", len(result.TrainTimetables), result.Count)
		}

		td, _ := time.Parse("2006-01-02", result.TrainDate)
		tdRef := fstore.Collection("trainDates").Doc(td.Format("20060102"))
		if _, err := tdRef.Set(context.Background(), map[string]interface{}{
			"publishTime": func() time.Time {
				t, _ := time.Parse(time.RFC3339, result.UpdateTime)
				return t
			}(),
			"updateTime": firestore.ServerTimestamp,
		}); err != nil {
			return fmt.Errorf("error writing trainDate: %s", err)
		}

		var batches []*firestore.WriteBatch
		for _, ttt := range result.TrainTimetables {
			tr := &Train{
				ID:        ttt.TrainInfo.TrainNo,
				Direction: &ttt.TrainInfo.Direction,
				Destination: &Station{
					ID:   ttt.TrainInfo.EndingStationID,
					Name: ttt.TrainInfo.EndingStationName.ZhTW,
				},
				TrainType: &TrainType{
					ID:   ttt.TrainInfo.TrainTypeID,
					Name: ttt.TrainInfo.TrainTypeName.ZhTW,
					Code: ttt.TrainInfo.TrainTypeCode,
				},
			}

			lastDepart, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT00:00:00+08:00", result.TrainDate))

			batch := fstore.Batch()
			for _, stop := range ttt.StopTimes {
				at, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s:00+08:00", result.TrainDate, stop.ArrivalTime))
				dt, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s:00+08:00", result.TrainDate, stop.DepartureTime))

				if at.Before(lastDepart) {
					at = at.Add(24 * time.Hour)
				}
				if dt.Before(at) {
					dt = dt.Add(24 * time.Hour)
				}
				lastDepart = dt

				ts := TrainStop{
					Train:        tr,
					StopSequence: stop.StopSequence,
					Station: &Station{
						ID:   stop.StationID,
						Name: stop.StationName.ZhTW,
					},
					Arrive: &StopTime{Time: at, Text: stop.ArrivalTime},
					Depart: &StopTime{Time: dt, Text: stop.DepartureTime},
				}

				tsRef := tdRef.Collection("trainStops").Doc(fmt.Sprintf("%s_%03d", ts.Train.ID, ts.StopSequence))
				batch = batch.Set(tsRef, ts)
			}

			batches = append(batches, batch)
		}

		processBatch := func(b *firestore.WriteBatch) error {
			if _, err := b.Commit(context.Background()); err != nil {
				return err
			}
			return nil
		}

		for _, b := range batches {
			if err := processBatch(b); err != nil {
				return fmt.Errorf("error writing trainStops: %s", err)
			}
		}

		log.Printf("Finish writing for %s with %v trains", result.TrainDate, result.Count)
		return nil
	}

	var wg sync.WaitGroup
	for _, date := range dates {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			fetch(d)
		}(date)
	}

	wg.Wait()

	return nil
}
