package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
	"github.com/news-maily/api/emails"
	"github.com/news-maily/api/queue"

	"github.com/news-maily/api/consumers"

	"github.com/news-maily/api/entities"
	"github.com/sirupsen/logrus"

	"github.com/news-maily/api/storage"
	"github.com/nsqio/go-nsq"
)

type MessageHandler struct {
	s storage.Storage
	p queue.Producer
}

// HandleMessage is the only requirement needed to fulfill the
// nsq.Handler interface.
func (h *MessageHandler) HandleMessage(m *nsq.Message) error {
	if len(m.Body) == 0 {
		return errors.New("body is blank")
	}

	msg := new(entities.SendCampaignParams)

	err := json.Unmarshal(m.Body, msg)
	if err != nil {
		return err
	}

	// fetching subs that are active and that have not been blacklisted
	var nextID int64
	var limit int64 = 1000
	for {
		subs, err := h.s.GetDistinctSubscribersByListIDs(msg.ListIDs, msg.UserID, false, true, nextID, limit)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"user_id":  msg.UserID,
				"list_ids": msg.ListIDs,
			}).Errorf("unable to fetch subscribers: %s", err.Error())
			break
		}

		logrus.Infof("subs count %d", len(subs))

		if len(subs) == 0 {
			break
		}

		// SES allows to send 50 emails in a bulk sending operation
		chunkSize := 50
		for i := 0; i < len(subs); i += chunkSize {
			end := i + chunkSize
			if end > len(subs) {
				end = len(subs)
			}

			// create
			var dest []*ses.BulkEmailDestination
			for _, s := range subs[i:end] {
				// marshal sub template data
				s.Normalize()
				td, err := json.Marshal(s.TemplateData)
				if err != nil {
					logrus.Errorf("unable to marshal template data for subscriber %d - %s", s.Id, err.Error())
					continue
				}

				d := &ses.BulkEmailDestination{
					Destination: &ses.Destination{
						ToAddresses: []*string{aws.String(s.Email)},
					},
					ReplacementTemplateData: aws.String(string(td)),
				}

				dest = append(dest, d)
			}

			uuid, err := uuid.NewRandom()
			if err != nil {
				logrus.Errorf("unable to generate random uuid: %s", err.Error())
				continue
			}

			defaultData, err := json.Marshal(msg.TemplateData)
			if err != nil {
				logrus.Errorln(err)
				continue
			}

			// prepare message for publishing to the queue
			msg, err := json.Marshal(entities.BulkSendMessage{
				UUID: uuid.String(),
				Input: &ses.SendBulkTemplatedEmailInput{
					Source:               aws.String(msg.Source),
					Template:             aws.String(msg.TemplateName),
					Destinations:         dest,
					ConfigurationSetName: aws.String(emails.ConfigurationSetName),
					DefaultTemplateData:  aws.String(string(defaultData)),
					DefaultTags: []*ses.MessageTag{
						&ses.MessageTag{
							Name:  aws.String("campaign_id"),
							Value: aws.String(strconv.Itoa(int(msg.CampaignID))),
						},
						&ses.MessageTag{
							Name:  aws.String("user_id"),
							Value: aws.String(strconv.Itoa(int(msg.UserID))),
						},
					},
				},
				CampaignID: msg.CampaignID,
				UserID:     msg.UserID,
				SesKeys:    &msg.SesKeys,
			})

			if err != nil {
				logrus.Errorln(err)
				continue
			}

			// publish the message to the queue
			err = h.p.Publish(entities.SendBulkTopic, msg)
			if err != nil {
				logrus.Errorln(err)
			}
		}

		nextID = subs[len(subs)-1].Id
	}

	err = h.s.UpdateCampaign(&entities.Campaign{
		Id:     msg.CampaignID,
		UserId: msg.UserID,
		Status: entities.StatusSent,
	})
	if err != nil {
		logrus.WithField("campaign_id", msg.CampaignID).Errorln(err)
	}

	return nil
}

func main() {
	lvl, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logrus.Panic(err)
	}

	logrus.SetLevel(lvl)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)

	s := storage.New(os.Getenv("DATABASE_DRIVER"), os.Getenv("DATABASE_CONFIG"))

	p, err := queue.NewProducer(os.Getenv("NSQD_HOST"), os.Getenv("NSQD_PORT"))
	if err != nil {
		logrus.Panic(err)
	}

	config := nsq.NewConfig()

	consumer, err := nsq.NewConsumer(entities.CampaignsTopic, entities.CampaignsTopic, config)
	if err != nil {
		log.Fatal(err)
	}

	consumer.ChangeMaxInFlight(200)

	consumer.SetLogger(
		&consumers.NoopLogger{},
		nsq.LogLevelError,
	)

	consumer.AddConcurrentHandlers(
		&MessageHandler{s, p},
		20,
	)

	addr := fmt.Sprintf("%s:%s", os.Getenv("NSQLOOKUPD_HOST"), os.Getenv("NSQLOOKUPD_PORT"))
	nsqlds := []string{addr}
	if err := consumer.ConnectToNSQLookupds(nsqlds); err != nil {
		log.Fatal(err)
	}

	shutdown := make(chan os.Signal, 2)
	signal.Notify(shutdown, os.Interrupt)
	signal.Notify(shutdown, syscall.SIGINT)
	signal.Notify(shutdown, syscall.SIGTERM)

	for {
		select {
		case <-consumer.StopChan:
			return // consumer disconnected. Time to quit.
		case <-shutdown:
			// Synchronously drain the queue before falling out of main
			consumer.Stop()
		}
	}
}
