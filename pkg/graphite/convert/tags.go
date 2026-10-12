package convert

import (
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

const (
	TaggedMetricName   = "graphite_tagged"
	UntaggedMetricName = "graphite_untagged"
)

func LabelsFromUntaggedName(name string, builder *labels.Builder) labels.Labels {
	builder.Reset(labels.EmptyLabels())

	for i, node := range strings.Split(name, ".") {
		builder.Set(fmt.Sprintf("__n%03d__", i), node)
	}

	builder.Set("__name__", UntaggedMetricName)
	return builder.Labels()
}
