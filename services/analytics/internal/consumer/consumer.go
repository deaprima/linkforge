// Package consumer memproses event link.clicked dari Kafka ke database analytics.
package consumer

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/enrich"
	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	eventspb "github.com/deaprima/linkforge/services/shared/proto/events"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

const defaultRetryBackoff = time.Second

// MessageReader adalah subset *kafka.Reader (consumer group) yang dipakai consumer.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}

type ClickStore interface {
	SaveClick(ctx context.Context, click *entity.LinkClick) (inserted bool, err error)
}

type Stats struct {
	Processed  int64 // klik baru tersimpan
	Duplicates int64 // event yang sudah pernah diproses (redelivery)
	Skipped    int64 // event rusak / tidak valid
}

// Consumer menjamin at-least-once + idempotent = setiap klik tercatat tepat sekali:
//   - offset hanya di-commit SETELAH klik tersimpan (atau dipastikan duplikat)
//   - error database → event yang sama di-retry terus, tidak lanjut ke pesan berikutnya
//     (commit offset yang lebih besar akan ikut meng-commit pesan yang gagal)
//   - event rusak (gagal decode) → di-skip & di-commit, supaya tidak macet selamanya
type Consumer struct {
	reader       MessageReader
	store        ClickStore
	enricher     *enrich.Enricher
	hub          *realtime.Hub
	retryBackoff time.Duration

	processed, duplicates, skipped atomic.Int64
}

func New(reader MessageReader, store ClickStore, enricher *enrich.Enricher, hub *realtime.Hub) *Consumer {
	return &Consumer{reader: reader, store: store, enricher: enricher, hub: hub, retryBackoff: defaultRetryBackoff}
}

func (c *Consumer) Stats() Stats {
	return Stats{Processed: c.processed.Load(), Duplicates: c.duplicates.Load(), Skipped: c.skipped.Load()}
}

// Run memproses event sampai ctx dibatalkan.
func (c *Consumer) Run(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("fetch message failed", "error", err)
			if !c.sleep(ctx) {
				return
			}
			continue
		}

		if err := c.handle(ctx, msg); err != nil {
			return // hanya terjadi saat shutdown; pesan belum di-commit dan akan dikirim ulang
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			// Aman: pesan akan dikirim ulang dan di-skip sebagai duplikat
			slog.Error("commit offset failed", "partition", msg.Partition, "offset", msg.Offset, "error", err)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg kafkago.Message) error {
	click, userID, err := c.decode(msg)
	if err != nil {
		c.skipped.Add(1)
		slog.Warn("skipping invalid event", "partition", msg.Partition, "offset", msg.Offset, "error", err)
		return nil
	}

	for {
		inserted, err := c.store.SaveClick(ctx, click)
		if err == nil {
			if !inserted {
				c.duplicates.Add(1)
				return nil
			}
			c.processed.Add(1)
			c.hub.Publish(realtime.Click{
				UserID:    userID,
				LinkID:    click.LinkID.String(),
				Alias:     click.Alias,
				Country:   click.Country,
				Browser:   click.Browser,
				Device:    click.Device,
				ClickedAt: click.ClickedAt,
			})
			return nil
		}
		slog.Error("save click failed, retrying", "event_id", click.EventID, "error", err)
		if !c.sleep(ctx) {
			return ctx.Err()
		}
	}
}

func (c *Consumer) decode(msg kafkago.Message) (*entity.LinkClick, string, error) {
	var ev eventspb.LinkClicked
	if err := proto.Unmarshal(msg.Value, &ev); err != nil {
		return nil, "", err
	}
	eventID, err := uuid.Parse(ev.GetEventId())
	if err != nil {
		return nil, "", errors.New("invalid event_id")
	}
	linkID, err := uuid.Parse(ev.GetLinkId())
	if err != nil {
		return nil, "", errors.New("invalid link_id")
	}
	if ev.GetClickedAt() == nil {
		return nil, "", errors.New("missing clicked_at")
	}

	info := c.enricher.Enrich(ev.GetIp(), ev.GetUserAgent())
	return &entity.LinkClick{
		EventID:   eventID,
		LinkID:    linkID,
		Alias:     ev.GetAlias(),
		IP:        ev.GetIp(),
		Country:   info.Country,
		Browser:   info.Browser,
		OS:        info.OS,
		Device:    info.Device,
		Referrer:  ev.GetReferrer(),
		ClickedAt: ev.GetClickedAt().AsTime().UTC(),
	}, ev.GetUserId(), nil
}

func (c *Consumer) sleep(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(c.retryBackoff):
		return true
	}
}
