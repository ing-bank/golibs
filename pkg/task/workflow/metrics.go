package workflow

import "github.com/prometheus/client_golang/prometheus"

var (
	workflowActivityStarted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_activity_started_total",
			Help: "Total number of workflow activities started.",
		},
		[]string{"workflow", "activity"},
	)
	workflowActivitySucceeded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_activity_succeeded_total",
			Help: "Total number of workflow activities succeeded.",
		},
		[]string{"workflow", "activity"},
	)
	workflowActivityFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_activity_failed_total",
			Help: "Total number of workflow activities failed.",
		},
		[]string{"workflow", "activity"},
	)
	workflowActivityDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "golibs_workflow_activity_duration_seconds",
			Help:    "Duration of workflow activities in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"workflow", "activity"},
	)

	workflowExecuteStarted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_execute_started_total",
			Help: "Total number of workflow executions started.",
		},
		[]string{"workflow", "concurrent"},
	)
	workflowExecuteSucceeded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_execute_succeeded_total",
			Help: "Total number of workflow executions succeeded.",
		},
		[]string{"workflow", "concurrent"},
	)
	workflowExecuteFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "golibs_workflow_execute_failed_total",
			Help: "Total number of workflow executions failed.",
		},
		[]string{"workflow", "concurrent"},
	)
	workflowExecuteDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "golibs_workflow_execute_duration_seconds",
			Help:    "Duration of workflow executions in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"workflow", "concurrent"},
	)
)

func init() {
	prometheus.MustRegister(workflowActivityStarted)
	prometheus.MustRegister(workflowActivitySucceeded)
	prometheus.MustRegister(workflowActivityFailed)
	prometheus.MustRegister(workflowActivityDuration)
	prometheus.MustRegister(workflowExecuteStarted)
	prometheus.MustRegister(workflowExecuteSucceeded)
	prometheus.MustRegister(workflowExecuteFailed)
	prometheus.MustRegister(workflowExecuteDuration)
}
