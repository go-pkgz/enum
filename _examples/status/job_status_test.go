package status

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobStatus(t *testing.T) {

	t.Run("basic", func(t *testing.T) {
		s := JobStatusActive
		assert.Equal(t, "active", s.String())
	})

	t.Run("json", func(t *testing.T) {
		type Data struct {
			Status JobStatus `json:"status"`
		}

		d := Data{Status: JobStatusActive}
		b, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, `{"status":"active"}`, string(b))

		var d2 Data
		err = json.Unmarshal([]byte(`{"status":"inactive"}`), &d2)
		require.NoError(t, err)
		assert.Equal(t, JobStatusInactive, d2.Status)
	})

	t.Run("sql", func(t *testing.T) {
		s := JobStatusActive

		// test Value() method
		v, err := s.Value()
		require.NoError(t, err)
		assert.Equal(t, "active", v)

		// test Scan from string
		var s2 JobStatus
		err = s2.Scan("inactive")
		require.NoError(t, err)
		assert.Equal(t, JobStatusInactive, s2)

		// test Scan from []byte
		err = s2.Scan([]byte("blocked"))
		require.NoError(t, err)
		assert.Equal(t, JobStatusBlocked, s2)

		// test Scan from nil - should get first value from StatusValues()
		err = s2.Scan(nil)
		require.NoError(t, err)
		assert.Equal(t, JobStatusValues()[0], s2)

		// test invalid value
		err = s2.Scan(123)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid jobStatus value")
	})

	t.Run("sqlite", func(t *testing.T) {
		db, err := sql.Open("sqlite", ":memory:")
		require.NoError(t, err)
		defer db.Close()

		// create table with status column
		_, err = db.Exec(`CREATE TABLE test_status (id INTEGER PRIMARY KEY, status TEXT)`)
		require.NoError(t, err)

		// insert different status values
		statuses := []JobStatus{JobStatusActive, JobStatusInactive, JobStatusBlocked}
		for i, s := range statuses {
			_, err = db.Exec(`INSERT INTO test_status (id, status) VALUES (?, ?)`, i+1, s)
			require.NoError(t, err)
		}

		// insert nil status
		_, err = db.Exec(`INSERT INTO test_status (id, status) VALUES (?, ?)`, 4, nil)
		require.NoError(t, err)

		// read and verify each status
		for i, expected := range statuses {
			var s JobStatus
			err = db.QueryRow(`SELECT status FROM test_status WHERE id = ?`, i+1).Scan(&s)
			require.NoError(t, err)
			assert.Equal(t, expected, s)
		}

		// verify nil status gets first value
		var s JobStatus
		err = db.QueryRow(`SELECT status FROM test_status WHERE id = 4`).Scan(&s)
		require.NoError(t, err)
		assert.Equal(t, JobStatusValues()[0], s)
	})

	t.Run("invalid", func(t *testing.T) {
		var d struct {
			Status JobStatus `json:"status"`
		}
		err := json.Unmarshal([]byte(`{"status":"invalid"}`), &d)
		assert.Error(t, err)
	})

	t.Run("iterator", func(t *testing.T) {
		var collected []JobStatus
		JobStatusIter()(func(js JobStatus) bool {
			collected = append(collected, js)
			return true
		})

		assert.Equal(t, JobStatusValues(), collected)

		collected = nil
		count := 0
		JobStatusIter()(func(js JobStatus) bool {
			collected = append(collected, js)
			count++
			return count < 2 // stop after collecting 2 items
		})

		assert.Equal(t, JobStatusValues()[:2], collected)
	})
}

func ExampleJobStatus() {
	s := JobStatusActive
	fmt.Println(s.String())
	// output: active
}

func ExampleJobStatusIter() {
	// using Go 1.23 range-over-func feature
	var allStatuses []JobStatus
	for js := range JobStatusIter() {
		allStatuses = append(allStatuses, js)
	}
	fmt.Println("All job statuses:", len(allStatuses))

	// early termination example
	var firstTwo []JobStatus
	count := 0
	for js := range JobStatusIter() {
		firstTwo = append(firstTwo, js)
		count++
		if count >= 2 {
			break
		}
	}
	fmt.Println("First two job statuses:", firstTwo[0], firstTwo[1])
	// output:
	// all job statuses: 4
	// first two job statuses: active blocked
}
