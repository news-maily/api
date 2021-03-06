package exporters

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/mailbadger/app/entities"
)

var (
	ErrUnknownResource = errors.New("unknown resource")
)

// Exporter represents type for creating exporters for different resource
type Exporter interface {
	Export(c context.Context, userID int64, report *entities.Report) error
}

func NewExporter(resource string, s3 s3iface.S3API) (Exporter, error) {
	switch resource {
	case "subscribers":
		return NewSubscribersExporter(s3), nil
	default:
		return nil, ErrUnknownResource
	}
}
