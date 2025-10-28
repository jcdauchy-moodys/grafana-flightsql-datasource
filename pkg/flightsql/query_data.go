package flightsql

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"google.golang.org/grpc/metadata"
)

// QueryData executes batches of ad-hoc queries and returns a batch of results.
func (d *FlightSQLDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	var (
		wg             sync.WaitGroup
		response       = backend.NewQueryDataResponse()
		executeResults = make(chan executeResult, len(req.Queries))
	)

	for _, dataQuery := range req.Queries {
		query, hostOverride, err := decodeQueryRequest(dataQuery)
		if err != nil {
			response.Responses[dataQuery.RefID] = backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			executeResults <- executeResult{
				refID:        query.RefID,
				dataResponse: d.query(ctx, *query, hostOverride),
			}
		}()
	}

	wg.Wait()
	close(executeResults)
	for r := range executeResults {
		response.Responses[r.refID] = r.dataResponse
	}

	return response, nil
}

// decodeQueryRequest decodes a [backend.DataQuery] and returns a
// [*sqlutil.Query] where all macros are expanded, along with the hostOverride if present.
func decodeQueryRequest(dataQuery backend.DataQuery) (*sqlutil.Query, string, error) {
	var q queryRequest
	if err := json.Unmarshal(dataQuery.JSON, &q); err != nil {
		return nil, "", fmt.Errorf("unmarshal json: %w", err)
	}

	var format sqlutil.FormatQueryOption
	switch q.Format {
	case "time_series":
		format = sqlutil.FormatOptionTimeSeries
	case "table":
		format = sqlutil.FormatOptionTable
	default:
		format = sqlutil.FormatOptionTimeSeries
	}

	query := &sqlutil.Query{
		RawSQL:        q.Text,
		RefID:         q.RefID,
		MaxDataPoints: q.MaxDataPoints,
		Interval:      time.Duration(q.IntervalMilliseconds) * time.Millisecond,
		TimeRange:     dataQuery.TimeRange,
		Format:        format,
	}

	// Process macros and execute the query.
	sql, err := sqlutil.Interpolate(query, macros)
	if err != nil {
		return nil, "", fmt.Errorf("macro interpolation: %w", err)
	}
	query.RawSQL = sql

	return query, q.HostOverride, nil
}

// executeResult is an envelope for concurrent query responses.
type executeResult struct {
	refID        string
	dataResponse backend.DataResponse
}

// queryRequest is an inbound query request as part of a batch of queries sent
// to [(*FlightSQLDatasource).QueryData].
type queryRequest struct {
	RefID                string `json:"refId"`
	Text                 string `json:"queryText"`
	IntervalMilliseconds int    `json:"intervalMs"`
	MaxDataPoints        int64  `json:"maxDataPoints"`
	Format               string `json:"format"`
	HostOverride         string `json:"hostOverride"`
}

// query executes a SQL statement by issuing a `CommandStatementQuery` command to Flight SQL.
// If hostOverride is provided, it will create a temporary client for that host instead of using the datasource's default client.
func (d *FlightSQLDatasource) query(ctx context.Context, query sqlutil.Query, hostOverride string) (resp backend.DataResponse) {
	defer func() {
		if r := recover(); r != nil {
			logErrorf("Panic: %s %s", r, string(debug.Stack()))
			resp = backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("panic: %s", r))
		}
	}()

	// Skip execution if query is empty
	if query.RawSQL == "" {
		return backend.DataResponse{}
	}

	// Determine which client to use
	clientToUse := d.client
	var tempClient *client

	if hostOverride != "" && hostOverride != d.getConfigAddr() {
		// Create a temporary client with the overridden host
		overrideCfg, err := d.createOverrideConfig(hostOverride)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("invalid host override: %s", err))
		}

		tempClient, err = newFlightSQLClient(overrideCfg)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("failed to create client for host override: %s", err))
		}
		defer tempClient.Close()

		clientToUse = tempClient
	}

	if d.md.Len() != 0 {
		ctx = metadata.NewOutgoingContext(ctx, d.md)
	}

	info, err := clientToUse.Execute(ctx, query.RawSQL)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("flightsql: %s", err))
	}
	if len(info.Endpoint) != 1 {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("unsupported endpoint count in response: %d", len(info.Endpoint)))
	}
	reader, err := clientToUse.DoGetWithHeaderExtraction(ctx, info.Endpoint[0].Ticket)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("flightsql: %s", err))
	}
	defer reader.Release()

	headers, err := reader.Header()
	if err != nil {
		logErrorf("Failed to extract headers: %s", err)
	}

	return newQueryDataResponse(reader, query, headers)
}
