package entities

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	valid "github.com/asaskevich/govalidator"
	"github.com/news-maily/app/utils"
)

// Subscriber represents the subscriber entity
type Subscriber struct {
	Model
	UserID      int64             `json:"-" gorm:"column:user_id; index"`
	Name        string            `json:"name"`
	Email       string            `json:"email" gorm:"not null"`
	MetaJSON    JSON              `json:"metadata" gorm:"column:metadata; type:json"`
	Segments    []Segment         `json:"-" gorm:"many2many:subscribers_segments;"`
	Blacklisted bool              `json:"blacklisted"`
	Active      bool              `json:"active"`
	Errors      map[string]string `json:"-" sql:"-"`
	Metadata    map[string]string `json:"-" sql:"-"`
}

// AppendUnsubscribeURLToMeta generates and signs a token based on the subscriber ID
// and appends an unsubscribe url with the email and token as query parameters, to the
// json metadata.
func (s *Subscriber) AppendUnsubscribeURLToMeta() error {
	m := make(map[string]string)

	if !s.MetaJSON.IsNull() {
		err := json.Unmarshal(s.MetaJSON, &m)
		if err != nil {
			return err
		}
	}

	t, err := s.GenerateUnsubscribeToken(os.Getenv("UNSUBSCRIBE_SECRET"))
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Add("email", s.Email)
	params.Add("token", t)

	m["unsubscribe_url"] = os.Getenv("APP_URL") + "/unsubscribe?" + params.Encode()

	jsonMeta, err := json.Marshal(m)
	if err != nil {
		return err
	}

	s.MetaJSON = jsonMeta

	return nil
}

// Validate subscriber properties,
func (s *Subscriber) Validate() bool {
	s.Errors = make(map[string]string)

	if len(s.Name) > 0 { // Name is optional
		if valid.Trim(s.Name, "") == "" {
			s.Errors["name"] = "The subscriber name cannot be empty."
		}

		if !valid.StringLength(s.Name, "1", "191") {
			s.Errors["name"] = "The name needs to be shorter than 190 characters."
		}
	}

	if !valid.IsEmail(s.Email) {
		s.Errors["email"] = "The specified email is not valid."
	}

	for key := range s.Metadata {
		if !valid.Matches(key, "^[\\w-]*$") {
			s.Errors["message"] = fmt.Sprintf("The specified key %s must consist only of alphanumeric and hyphen characters", key)
			break
		}
	}

	return len(s.Errors) == 0
}

// GenerateUnsubscribeToken generates and signs a new unsubscribe token with the given secret, from the
// ID of the subscriber. When a subscriber wants to unsubscribe from future emails, we check this hash
// against a newly generated hash and compare them, if they match we unsubscribe the user.
func (s *Subscriber) GenerateUnsubscribeToken(secret string) (string, error) {
	if s.ID == 0 {
		return "", errors.New("entities: unable to generate unsubscribe token: subscriber ID is 0")
	}

	if secret == "" {
		return "", errors.New("entities: unable to generate unsubscribe token: secret is empty")
	}

	return utils.SignData(strconv.FormatInt(s.ID, 10), secret)
}

func (s Subscriber) GetID() int64 {
	return s.Model.ID
}

func (s Subscriber) GetCreatedAt() time.Time {
	return s.Model.CreatedAt
}

func (s Subscriber) GetUpdatedAt() time.Time {
	return s.Model.UpdatedAt
}
