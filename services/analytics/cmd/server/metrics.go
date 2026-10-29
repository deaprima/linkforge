package main

import (
	"github.com/deaprima/linkforge/services/analytics/internal/consumer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	kafkago "github.com/segmentio/kafka-go"
)

// registerConsumerMetrics mengekspos kesehatan consumer Kafka: lag (selisih offset terbaru broker
// dengan yang sudah dibaca) dan hasil pemrosesan event. Lag yang terus naik = consumer tidak
// mengejar producer.
func registerConsumerMetrics(reader *kafkago.Reader, c *consumer.Consumer, topic, group string) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "kafka_consumer_lag",
		Help:        "Jumlah pesan yang belum dibaca consumer group (dari reader stats kafka-go).",
		ConstLabels: prometheus.Labels{"topic": topic, "group": group},
	}, func() float64 { return float64(reader.Stats().Lag) })

	for result, read := range map[string]func(consumer.Stats) int64{
		"processed": func(s consumer.Stats) int64 { return s.Processed },
		"duplicate": func(s consumer.Stats) int64 { return s.Duplicates },
		"skipped":   func(s consumer.Stats) int64 { return s.Skipped },
	} {
		promauto.NewCounterFunc(prometheus.CounterOpts{
			Name:        "analytics_click_events_total",
			Help:        "Event link.clicked yang diproses consumer, per hasil (processed, duplicate, skipped).",
			ConstLabels: prometheus.Labels{"result": result},
		}, func() float64 { return float64(read(c.Stats())) })
	}
}
