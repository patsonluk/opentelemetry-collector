// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loggingexporter // import "go.opentelemetry.io/collector/exporter/loggingexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/exporter/loggingexporter/internal/otlptext"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	url2 "net/url"
	"os"
	"strings"
	"time"
)

type loggingExporter struct {
	verbosity        configtelemetry.Level
	logger           *zap.Logger
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
	tracesMarshaler  ptrace.Marshaler
}

var (
	appKey = os.Getenv("FS_APP_KEY")
)

func (s *loggingExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	s.logger.Info("TracesExporter", zap.Int("#spans", td.SpanCount()))
	if s.verbosity != configtelemetry.LevelDetailed {
		return nil
	}

	buf, err := s.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				s.exportFsServerEvent(span)

			}
		}
	}

	return nil
}

func getFsIds(tracestate string) (string, string) {
	printRed("Raw tracestate %s", tracestate)
	for _, vendorToken := range strings.Split(tracestate, ",") {
		tokens := strings.SplitN(vendorToken, "=", 2)
		//printRed("vendor token %s splits into %v", vendorToken, tokens)
		if tokens[0] == "fs" {
			ids := strings.SplitN(tokens[1], ":", 2) //in format of uid:session_url
			return ids[0], ids[1]
		}
	}
	return "", ""
}

const colorRed = "\033[0;31m"
const colorNone = "\033[0m"

func printRed(format string, args ...any) {
	s := fmt.Sprintf(format, args)
	fmt.Printf("%s%s%s\n", colorRed, s, colorNone)
}

func (s *loggingExporter) exportFsServerEvent(span ptrace.Span) error {
	//fs=test-user-1:https://app.staging.fullstory.com/ui/6XMR/session/4547941319770112%3A5472192830832640
	//fmt.Printf("FS TRACE STATE: %s\n", span.TraceState().AsRaw())
	uid, sessionUrl := getFsIds(span.TraceState().AsRaw())
	if uid == "" || sessionUrl == "" {
		printRed("cannot extract uid/session_url from the tracestate %s", span.TraceState())
		return nil
	}
	url := fmt.Sprintf("https://api.staging.fullstory.com/users/v1/individual/%s/customevent", url2.QueryEscape(uid))

	printRed("URL will be %s\n", url)

	reqs, err := convertSpanIntoEventReqs(span, sessionUrl)
	if err != nil {
		printRed("Failed to convert span into reqs %s", err.Error())
		return nil
	}
	for _, req := range reqs {
		postServerEvent(url, req)
	}
	return nil
}

func postServerEvent(url string, req []byte) error {
	printRed("json string %s", string(req))
	request, error := http.NewRequest("POST", url, bytes.NewBuffer(req))
	if error != nil {
		return error
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", "Basic "+appKey)

	client := &http.Client{}
	response, error := client.Do(request)
	defer response.Body.Close()

	fmt.Println("FS response Status:", response.Status)
	fmt.Println("response Headers:", response.Header)
	body, _ := ioutil.ReadAll(response.Body)
	fmt.Println("response Body:", string(body))
	return nil
}

func convertSpanIntoEventReqs(span ptrace.Span, sessionUrl string) ([][]byte, error) {
	now := time.Now().UnixMilli()
	var reqJsons []map[string]interface{}

	startName := fmt.Sprintf("OT start - ts %d", now)
	startTs := span.StartTimestamp().AsTime().Format("2006-01-02T15:04:05.000Z")
	attributes := span.Attributes()
	reqJsons = append(reqJsons, createBasicJson(startName, sessionUrl, startTs, &attributes))
	//TODO log as events

	endName := fmt.Sprintf("OT end - ts %d", now)
	endTs := span.EndTimestamp().AsTime().Format("2006-01-02T15:04:05.000Z")
	reqJsons = append(reqJsons, createBasicJson(endName, sessionUrl, endTs, nil))

	var result [][]byte
	for _, reqJson := range reqJsons {
		b, err := json.Marshal(reqJson)
		if err != nil {
			fmt.Errorf("json marshal error %s", err.Error())
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

func createBasicJson(eventName string, sessionUrl string, timestamp string, attributes *pcommon.Map) map[string]interface{} {
	var reqJson = make(map[string]interface{})
	eventJson := make(map[string]interface{})
	reqJson["event"] = eventJson

	printRed("event name : %s", eventName)
	eventJson["event_name"] = fmt.Sprintf(eventName)

	printRed("timestamp is %s\n", timestamp)
	eventJson["timestamp"] = timestamp
	eventJson["session_url"] = sessionUrl

	dataMap := make(map[string]interface{})
	if attributes != nil {
		attributes.Range(func(key string, value pcommon.Value) bool {
			if value.Type() == pcommon.ValueTypeStr {
				dataMap[key+"_str"] = value.Str()
			} else if value.Type() == pcommon.ValueTypeInt {
				dataMap[key+"_int"] = value.Int()
			} else {
				printRed("Unhandled span attribute type %v", value.Type())
			}
			return true
		})
	}
	dataMap["echo_timestamp_str"] = timestamp

	eventJson["event_data"] = dataMap

	return reqJson
}

func (s *loggingExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	s.logger.Info("MetricsExporter", zap.Int("#metrics", md.MetricCount()))
	if s.verbosity != configtelemetry.LevelDetailed {
		return nil
	}

	buf, err := s.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))
	return nil
}

func (s *loggingExporter) pushLogs(_ context.Context, ld plog.Logs) error {
	s.logger.Info("LogsExporter", zap.Int("#logs", ld.LogRecordCount()))
	if s.verbosity != configtelemetry.LevelDetailed {
		return nil
	}

	buf, err := s.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))
	return nil
}

func newLoggingExporter(logger *zap.Logger, verbosity configtelemetry.Level) *loggingExporter {
	return &loggingExporter{
		verbosity:        verbosity,
		logger:           logger,
		logsMarshaler:    otlptext.NewTextLogsMarshaler(),
		metricsMarshaler: otlptext.NewTextMetricsMarshaler(),
		tracesMarshaler:  otlptext.NewTextTracesMarshaler(),
	}
}

func loggerSync(logger *zap.Logger) func(context.Context) error {
	return func(context.Context) error {
		// Currently Sync() return a different error depending on the OS.
		// Since these are not actionable ignore them.
		err := logger.Sync()
		osErr := &os.PathError{}
		if errors.As(err, &osErr) {
			wrappedErr := osErr.Unwrap()
			if knownSyncError(wrappedErr) {
				err = nil
			}
		}
		return err
	}
}
