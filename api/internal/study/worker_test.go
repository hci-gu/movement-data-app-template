package study

import (
	"context"
	"database/sql"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
)

func TestWorkerPollingByEnvironment(t *testing.T) {
	for _, environment := range []string{"disabled", "test"} {
		t.Run(environment, func(t *testing.T) {
			s, _, _ := testService(t)
			s.Config.Environment = environment
			// Pausing new signing must still allow enabled workers to finish orders.
			s.Config.SigningEnabled = false
			var queries atomic.Int32
			s.App.ConcurrentDB().(*dbx.DB).QueryLogFunc = func(_ context.Context, _ time.Duration, query string, _ *sql.Rows, _ error) {
				if strings.Contains(query, "bankid_orders") {
					queries.Add(1)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 650*time.Millisecond)
			defer cancel()
			s.Run(ctx)
			if environment == "disabled" && queries.Load() != 0 {
				t.Fatalf("disabled worker queried BankID orders %d times", queries.Load())
			}
			if environment != "disabled" && queries.Load() == 0 {
				t.Fatal("enabled worker did not poll while new signing was paused")
			}
		})
	}
}
