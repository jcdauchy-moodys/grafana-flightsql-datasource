package flightsql

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apache/arrow/go/v12/arrow/flight"
	"github.com/apache/arrow/go/v12/arrow/flight/flightsql"
	"github.com/apache/arrow/go/v12/arrow/flight/flightsql/example"
	"github.com/apache/arrow/go/v12/arrow/memory"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_QueryData(t *testing.T) {
	db, err := example.CreateDB()
	require.NoError(t, err)
	defer db.Close()

	sqliteServer, err := example.NewSQLiteFlightSQLServer(db)
	require.NoError(t, err)
	sqliteServer.Alloc = memory.NewCheckedAllocator(memory.DefaultAllocator)
	server := flight.NewServerWithMiddleware(nil)
	server.RegisterFlightService(flightsql.NewFlightServer(sqliteServer))
	err = server.Init("localhost:0")
	require.NoError(t, err)
	go server.Serve()
	defer server.Shutdown()

	cfg := config{
		Addr:   server.Addr().String(),
		Token:  "secret",
		Secure: false,
	}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)

	settings := backend.DataSourceInstanceSettings{JSONData: cfgJSON}
	ds, err := NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	resp, err := ds.(*FlightSQLDatasource).QueryData(context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					JSON:  mustQueryJSON(t, "A", "select * from intTable"),
				},
				{
					RefID: "B",
					JSON:  mustQueryJSON(t, "B", "select 1"),
				},
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, resp.Responses, 2)

	respA := resp.Responses["A"]
	require.NoError(t, respA.Error)
	frame := respA.Frames[0]

	require.Equal(t, "id", frame.Fields[0].Name)
	require.Equal(t, "keyName", frame.Fields[1].Name)
	require.Equal(t, "value", frame.Fields[2].Name)
	require.Equal(t, "foreignId", frame.Fields[3].Name)
	for _, f := range frame.Fields {
		assert.Equal(t, 4, f.Len())
	}
}

func mustQueryJSON(t *testing.T, refID, sql string) []byte {
	t.Helper()

	b, err := json.Marshal(queryRequest{
		RefID:  refID,
		Text:   sql,
		Format: "table",
	})
	if err != nil {
		panic(err)
	}
	return b
}

func mustQueryJSONWithHostOverride(t *testing.T, refID, sql, hostOverride string) []byte {
	t.Helper()

	b, err := json.Marshal(queryRequest{
		RefID:        refID,
		Text:         sql,
		Format:       "table",
		HostOverride: hostOverride,
	})
	if err != nil {
		panic(err)
	}
	return b
}

func TestIntegration_QueryDataWithHostOverride(t *testing.T) {
	// Create first server
	db1, err := example.CreateDB()
	require.NoError(t, err)
	defer db1.Close()

	sqliteServer1, err := example.NewSQLiteFlightSQLServer(db1)
	require.NoError(t, err)
	sqliteServer1.Alloc = memory.NewCheckedAllocator(memory.DefaultAllocator)
	server1 := flight.NewServerWithMiddleware(nil)
	server1.RegisterFlightService(flightsql.NewFlightServer(sqliteServer1))
	err = server1.Init("localhost:0")
	require.NoError(t, err)
	go server1.Serve()
	defer server1.Shutdown()

	// Create second server for override testing
	db2, err := example.CreateDB()
	require.NoError(t, err)
	defer db2.Close()

	sqliteServer2, err := example.NewSQLiteFlightSQLServer(db2)
	require.NoError(t, err)
	sqliteServer2.Alloc = memory.NewCheckedAllocator(memory.DefaultAllocator)
	server2 := flight.NewServerWithMiddleware(nil)
	server2.RegisterFlightService(flightsql.NewFlightServer(sqliteServer2))
	err = server2.Init("localhost:0")
	require.NoError(t, err)
	go server2.Serve()
	defer server2.Shutdown()

	// Configure datasource to use server1
	cfg := config{
		Addr:   server1.Addr().String(),
		Token:  "secret",
		Secure: false,
	}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)

	settings := backend.DataSourceInstanceSettings{JSONData: cfgJSON}
	ds, err := NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	// Test 1: Query without override should use default server
	resp, err := ds.(*FlightSQLDatasource).QueryData(context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					JSON:  mustQueryJSON(t, "A", "select * from intTable"),
				},
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, resp.Responses["A"])
	assert.NoError(t, resp.Responses["A"].Error)

	// Test 2: Query with host override should use overridden server
	resp2, err := ds.(*FlightSQLDatasource).QueryData(context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{
					RefID: "B",
					JSON:  mustQueryJSONWithHostOverride(t, "B", "select * from intTable", server2.Addr().String()),
				},
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, resp2.Responses["B"])
	assert.NoError(t, resp2.Responses["B"].Error)

	// Test 3: Invalid host override should return error
	resp3, err := ds.(*FlightSQLDatasource).QueryData(context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{
					RefID: "C",
					JSON:  mustQueryJSONWithHostOverride(t, "C", "select * from intTable", "invalid-host"),
				},
			},
		})
	require.NoError(t, err)
	assert.NotNil(t, resp3.Responses["C"])
	assert.Error(t, resp3.Responses["C"].Error)
	assert.Contains(t, resp3.Responses["C"].Error.Error(), "host override must be in the form")
}
