// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.datum.net/o11y/queryapi/internal/storage"
)

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type lokiQueryResponse struct {
	Status string        `json:"status"`
	Data   lokiQueryData `json:"data"`
}

type lokiQueryData struct {
	ResultType string       `json:"resultType"`
	Result     []lokiStream `json:"result"`
}

// collectStreams drains iter, grouping rows into Loki streams. Limit bounds
// the row count, so the accumulator is bounded too.
func collectStreams(iter storage.LogIterator) (result []lokiStream, err error) {
	defer func() {
		err = errors.Join(err, iter.Close())
	}()

	byKey := map[string]int{}
	for iter.Next() {
		row := iter.Row()
		key := row.Labels.Key()
		i, ok := byKey[key]
		if !ok {
			i = len(result)
			byKey[key] = i
			result = append(result, lokiStream{Stream: row.Labels, Values: nil})
		}
		result[i].Values = append(result[i].Values, [2]string{
			strconv.FormatInt(row.Timestamp.UnixNano(), 10),
			row.Line,
		})
	}
	return result, iter.Err()
}

func writeStreams(w http.ResponseWriter, streams []lokiStream) {
	if streams == nil {
		streams = []lokiStream{}
	}
	writeJSON(w, lokiQueryResponse{
		Status: "success",
		Data:   lokiQueryData{ResultType: "streams", Result: streams},
	})
}

func writeData(w http.ResponseWriter, data any) {
	writeJSON(w, struct {
		Status string `json:"status"`
		Data   any    `json:"data"`
	}{Status: "success", Data: data})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}
