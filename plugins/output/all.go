package output

import (
	_ "github.com/mbiondo/logAnalyzer/plugins/output/console"
	_ "github.com/mbiondo/logAnalyzer/plugins/output/elasticsearch"
	_ "github.com/mbiondo/logAnalyzer/plugins/output/file"
	_ "github.com/mbiondo/logAnalyzer/plugins/output/prometheus"
	_ "github.com/mbiondo/logAnalyzer/plugins/output/slack"
)
