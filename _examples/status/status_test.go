package status

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		s := StatusActive
		assert.Equal(t, "active", s.String())
	})

	t.Run("json", func(t *testing.T) {
		type Data struct {
			Status Status `json:"status"`
		}

		d := Data{Status: StatusActive}
		b, err := json.Marshal(d)
		require.NoError(t, err)
		assert.Equal(t, `{"status":"active"}`, string(b))

		var d2 Data
		err = json.Unmarshal([]byte(`{"status":"inactive"}`), &d2)
		require.NoError(t, err)
		assert.Equal(t, StatusInactive, d2.Status)
	})

	t.Run("invalid", func(t *testing.T) {
		var d struct {
			Status Status `json:"status"`
		}
		err := json.Unmarshal([]byte(`{"status":"invalid"}`), &d)
		assert.Error(t, err)
	})
}

func ExampleStatus() {
	s := StatusActive
	fmt.Println(s.String())
	// Output: active
}
