package web

import (
	"log"
	"net/http"

	"github.com/CorySanin/swankyindex/internal/config"
	"github.com/CorySanin/swankyindex/pkg/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type DatabaseCollector struct {
	downloadTotals *prometheus.Desc
	store          *storage.Storage
	conf           *config.Conf
}

func (s *Server) MetricsHandler() http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),                                       // Metrics from Go runtime
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // Metrics about the current UNIX process
		NewDatabaseCollector(s.conf, s.store),                             // Download metrics
	)

	return promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
}

func NewDatabaseCollector(cfg *config.Conf, st *storage.Storage) *DatabaseCollector {
	return &DatabaseCollector{
		downloadTotals: prometheus.NewDesc(
			"swanky_download_totals",
			"Download counts of all files",
			[]string{"filename", "access_domain"}, nil,
		),
		conf:  cfg,
		store: st,
	}
}

func (c *DatabaseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.downloadTotals
}

func (c *DatabaseCollector) Collect(ch chan<- prometheus.Metric) {

	dbch := make(chan []storage.TotalsRow, 1)
	c.store.GetTotalsByFileAndAccessDomain(dbch)
	totals := <-dbch

	cdir, err := c.conf.GetDirectory()
	if err != nil {
		log.Panicf("directory was undefined: %v", err)
	}
	substr := len(cdir)

	for _, t := range totals {
		ch <- prometheus.MustNewConstMetric(
			c.downloadTotals,
			prometheus.CounterValue,
			float64(t.Count),
			t.Filename[substr:],
			t.AccessDomain,
		)
	}
}
