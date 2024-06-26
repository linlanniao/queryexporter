package collector

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/fengxsong/queryexporter/pkg/config"
	_ "github.com/fengxsong/queryexporter/pkg/querier"
	"github.com/fengxsong/queryexporter/pkg/querier/factory"
)

type queries struct {
	namespace string
	cfg       *config.Config
	logger    log.Logger

	scrapeDurationDesc *prometheus.Desc
	lock               sync.Mutex
}

func New(name string, cfg *config.Config, logger log.Logger) (prometheus.Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	qs := &queries{
		namespace: name,
		cfg:       cfg,
		logger:    logger,
		scrapeDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName(name, "", "scrape_duration"),
			"querier scrape duration",
			[]string{"driver", "metric", "success"}, nil,
		),
	}
	return qs, nil
}

func (q *queries) Describe(ch chan<- *prometheus.Desc) {
	ch <- q.scrapeDurationDesc
}

func (q *queries) Collect(ch chan<- prometheus.Metric) {
	q.lock.Lock()
	defer q.lock.Unlock()

	wg := &sync.WaitGroup{}
	ctx := context.Background()

	for driver, metrics := range q.cfg.Metrics {
		for i := range metrics {
			wg.Add(1)
			go func(subsystem string, metric *config.Metric) {
				defer wg.Done()
				start := time.Now()
				// TODO: do actual collect
				err := factory.Default.Process(ctx, q.logger, q.namespace, subsystem, metric.DataSources, metric.Metric, ch)
				if err != nil {
					level.Error(q.logger).Log("err", err)
				}
				ch <- prometheus.MustNewConstMetric(
					q.scrapeDurationDesc,
					prometheus.GaugeValue,
					time.Since(start).Seconds(),
					subsystem, metric.String(), strconv.FormatBool(err == nil))
			}(driver, metrics[i])
		}
	}
	wg.Wait()
}
