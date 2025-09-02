package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/go-pkgz/testutils/containers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

func TestGeneratedEnumWithMongoDB(t *testing.T) {
	ctx := context.Background()

	// start MongoDB container
	mongoContainer := containers.NewMongoTestContainer(ctx, t, 7)
	defer mongoContainer.Close(ctx)

	// get collection
	coll := mongoContainer.Collection("test_db")

	// test with generated Status enum
	type Doc struct {
		ID     string `bson:"_id"`
		Status Status `bson:"status"`
		Name   string `bson:"name"`
	}

	// insert with enum value
	doc := Doc{
		ID:     "test1",
		Status: StatusActive,
		Name:   "Test Document",
	}

	_, err := coll.InsertOne(ctx, doc)
	require.NoError(t, err, "should insert document with enum")

	// retrieve and verify
	var retrieved Doc
	err = coll.FindOne(ctx, bson.M{"_id": "test1"}).Decode(&retrieved)
	require.NoError(t, err, "should retrieve document")

	assert.Equal(t, StatusActive, retrieved.Status)
	assert.Equal(t, "active", retrieved.Status.String())

	// verify BSON storage format is string
	var raw bson.M
	err = coll.FindOne(ctx, bson.M{"_id": "test1"}).Decode(&raw)
	require.NoError(t, err)

	statusField, ok := raw["status"].(string)
	assert.True(t, ok, "status should be stored as string in BSON")
	assert.Equal(t, "active", statusField)
}

func TestGeneratedEnumWithSQL(t *testing.T) {
	// create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// create table
	_, err = db.Exec(`
		CREATE TABLE records (
			id INTEGER PRIMARY KEY,
			status TEXT,
			priority INTEGER
		)
	`)
	require.NoError(t, err)

	// test Status enum
	_, err = db.Exec("INSERT INTO records (id, status, priority) VALUES (?, ?, ?)",
		1, StatusActive, PriorityHigh)
	require.NoError(t, err)

	var status Status
	var priority Priority
	row := db.QueryRow("SELECT status, priority FROM records WHERE id = ?", 1)
	err = row.Scan(&status, &priority)
	require.NoError(t, err)

	assert.Equal(t, StatusActive, status)
	assert.Equal(t, "active", status.String())
	assert.Equal(t, PriorityHigh, priority)

	// test NULL handling
	_, err = db.Exec("INSERT INTO records (id, status, priority) VALUES (?, NULL, NULL)", 2)
	require.NoError(t, err)

	row = db.QueryRow("SELECT status, priority FROM records WHERE id = ?", 2)
	err = row.Scan(&status, &priority)
	require.NoError(t, err)
	assert.Equal(t, StatusUnknown, status) // zero value
	assert.Equal(t, PriorityLow, priority) // zero value for priority (0)
}

func TestGeneratedEnumWithYAML(t *testing.T) {
	// test Status enum
	status := StatusInactive
	data, err := yaml.Marshal(status)
	require.NoError(t, err)
	assert.Equal(t, "inactive\n", string(data))

	var unmarshaled Status
	err = yaml.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, StatusInactive, unmarshaled)

	// test in struct
	type Config struct {
		Status   Status   `yaml:"status"`
		Priority Priority `yaml:"priority"`
	}

	cfg := Config{
		Status:   StatusBlocked,
		Priority: PriorityCritical,
	}

	data, err = yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded Config
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, cfg, decoded)

	// test error handling
	var badStatus Status
	err = yaml.Unmarshal([]byte("invalid_status"), &badStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestGeneratedEnumWithJSON(t *testing.T) {
	// test Status enum
	status := StatusPending
	data, err := json.Marshal(status)
	require.NoError(t, err)
	assert.Equal(t, `"pending"`, string(data))

	var unmarshaled Status
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, unmarshaled)

	// test Priority enum
	priority := PriorityMedium
	data, err = json.Marshal(priority)
	require.NoError(t, err)

	var unmarshaledPriority Priority
	err = json.Unmarshal(data, &unmarshaledPriority)
	require.NoError(t, err)
	assert.Equal(t, PriorityMedium, unmarshaledPriority)

	// test error handling
	var badStatus Status
	err = json.Unmarshal([]byte(`"not_a_status"`), &badStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}
