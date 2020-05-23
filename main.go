package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

var (
	matcher = regexp.MustCompile(`^\/(\d+\:\w+)\/-?(\d+)\/$`)
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	// log.SetLevel(log.WarnLevel)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			jerr := json.NewEncoder(w).Encode(
				map[string]bool{
					"alive": true,
					"ready": true,
				},
			)
			if jerr != nil {
				log.Error(jerr)
				return
			}
			log.WithFields(log.Fields{
				"Method":     r.Method,
				"Header":     r.Header,
				"RemoteAddr": r.RemoteAddr,
				"RequestURI": r.RequestURI,
			}).Debug("health request served")
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(
				map[string]error{
					"error": err,
				},
			)
			if jerr != nil {
				log.Error(jerr)
				return
			}
			log.WithFields(log.Fields{
				"Header":     r.Header,
				"RemoteAddr": r.RemoteAddr,
				"RequestURI": r.RequestURI,
				"error_from": "ioutil.ReadAll(r.Body)",
			}).Warn("request rejected")
			return
		}

		matchResult := matcher.FindStringSubmatch(r.RequestURI)
		if len(matchResult) != 3 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(
				map[string]error{
					"error": err,
				},
			)
			if jerr != nil {
				log.Error(jerr)
				return
			}
			log.WithFields(log.Fields{
				"Header":      r.Header,
				"Body":        string(body),
				"RemoteAddr":  r.RemoteAddr,
				"RequestURI":  r.RequestURI,
				"matchResult": matchResult,
				"error_place": "len(matchResult) != 3",
			}).Warn("request rejected")
			return
		}

		var (
			posted interface{}
			ChatID int
		)

		err = json.Unmarshal(body, &posted)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(
				map[string]error{
					"error": err,
				},
			)
			if jerr != nil {
				log.Error(jerr)
				return
			}
			log.WithFields(log.Fields{
				"Header":      r.Header,
				"Body":        string(body),
				"RemoteAddr":  r.RemoteAddr,
				"RequestURI":  r.RequestURI,
				"matchResult": matchResult,
				"error_place": "json.Unmarshal(body, &posted)",
			}).Warn("request rejected")
			return
		}

		log.WithFields(log.Fields{
			"Header":      r.Header,
			"json":        posted,
			"Body":        string(body),
			"RemoteAddr":  r.RemoteAddr,
			"RequestURI":  r.RequestURI,
			"matchResult": matchResult,
		}).Info("request processed")

		if ChatID, err = strconv.Atoi(matchResult[2]); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(
				map[string]error{
					"error": err,
				},
			)
			if jerr != nil {
				log.Error(jerr)
				return
			}
			log.WithFields(log.Fields{
				"Header":      r.Header,
				"Body":        string(body),
				"RemoteAddr":  r.RemoteAddr,
				"RequestURI":  r.RequestURI,
				"matchResult": matchResult,
				"error_place": "strconv.Atoi(matchResult[2])",
			}).Info("request rejected")
			return
		}

		msg, err := sendMessage(
			matchResult[1],
			int64(ChatID),
			string(body),
		)

		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			jerr := json.NewEncoder(w).Encode(msg.Date)
			if jerr != nil {
				log.Error(jerr)
				return
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(err)
			if jerr != nil {
				log.Error(jerr)
				return
			}
		}
	})

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Error(err)
	}

}

func sendMessage(token string, id int64, message string) (tgbotapi.Message, error) {
	bot, err := tgbotapi.NewBotAPI(token)

	if err != nil {
		log.Panic(err)
	}

	// bot.Debug = true

	msg := tgbotapi.NewMessage(id, message)

	return bot.Send(msg)
}
