package fdb_test

import (
	"fmt"
	"iter"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

type TestDb struct {
	Tables []*fdb.Table
	Rows   map[string][]fdb.Row
}

func entry(variant fdb.Variant, data any) *fdb.DataEntry {
	return fdb.NewEntry(variant, data)
}

func checkTable(t *testing.T, expected, actual *fdb.Table) {
	if expected.Name != actual.Name {
		t.Errorf("%s: expected name %s but got %s", expected.Name, expected.Name, actual.Name)
	}

	if len(expected.Columns) != len(actual.Columns) {
		t.Errorf("%s: expected %d columns but got %d", expected.Name, len(expected.Columns), len(actual.Columns))
		return
	}

	for i, expectedCol := range expected.Columns {
		actualCol := actual.Columns[i]

		if expectedCol.Name != actualCol.Name {
			t.Errorf("%s: expected column %d to have name %s but got %s", expected.Name, i, expectedCol.Name, actualCol.Name)
		}

		if expectedCol.Variant != actualCol.Variant {
			t.Errorf("%s: expected column %d to have variant %v but got %v", expected.Name, i, expectedCol.Variant, actualCol.Variant)
		}
	}
}

func checkRow(t *testing.T, tableName string, expected, actual fdb.Row) {
	if len(expected) != len(actual) {
		t.Errorf("%s: expected %d entries but got %d", tableName, len(expected), len(actual))
		return
	}

	for i := range expected {
		if expected[i].Variant() != actual[i].Variant() {
			t.Errorf("%s: expected entry %d to have variant %v but got %v", tableName, i, expected[i].Variant(), actual[i].Variant())
			continue
		}

		expectedVal, err := expected.Value(i)
		if err != nil {
			t.Errorf("%s: %v", tableName, err)
			return
		}

		actualVal, err := expected.Value(i)
		if err != nil {
			t.Errorf("%s: %v", tableName, err)
			return
		}

		if expectedVal != actualVal {
			t.Errorf("%s: expected entry %d to have value %v but got %v", tableName, i, expectedVal, actualVal)
		}
	}
}

func checkRows(t *testing.T, expected []fdb.Row, expectedTable, actualTable *fdb.Table) {
	actual := []fdb.Row{}
	for row, err := range actualTable.Rows() {
		if err != nil {
			t.Errorf("%s: %v", actualTable.Name, err)
			return
		}

		actual = append(actual, row)
	}

	if len(expected) != len(actual) {
		t.Errorf("%s: expected %d rows but got %d", expectedTable.Name, expected, actual)
		return
	}

	for i, expectedRow := range expected {
		t.Logf("%s: checking row %d", expectedTable.Name, i)

		actualRow := actual[i]
		checkRow(t, expectedTable.Name, expectedRow, actualRow)
	}
}

func checkReader(t *testing.T, expectedDb TestDb, reader *fdb.Reader) {
	for _, expectedTable := range expectedDb.Tables {
		t.Logf("checking table %s", expectedTable.Name)

		actualTable, ok := reader.FindTable(expectedTable.Name)
		if !ok {
			t.Errorf("could not find table %s", expectedTable.Name)
			continue
		}

		checkTable(t, expectedTable, actualTable)
		checkRows(t, expectedDb.Rows[expectedTable.Name], expectedTable, actualTable)
	}
}

func testRead(fdbName string, expectedDb TestDb) func(*testing.T) {
	return func(t *testing.T) {
		reader, err := fdb.OpenReader(filepath.Join("testdata", fdbName))
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		checkReader(t, expectedDb, reader)
	}
}

func TestRead(t *testing.T) {
	t.Run("read_basic", testRead("basic.fdb", TestDb{
		Tables: []*fdb.Table{
			{Name: "Accounts", Columns: []*fdb.Column{{fdb.VariantU32, "id"}, {fdb.VariantNVarChar, "name"}, {fdb.VariantU32, "age"}, {fdb.VariantBool, "isActive"}}},
			{Name: "Skills", Columns: []*fdb.Column{{fdb.VariantU32, "accountId"}, {fdb.VariantText, "skillName"}, {fdb.VariantU64, "power"}}},
			{Name: "Version", Columns: []*fdb.Column{{fdb.VariantI32, "major"}, {fdb.VariantI32, "minor"}, {fdb.VariantI32, "patch"}}},
			{Name: "NPCs", Columns: []*fdb.Column{{fdb.VariantNVarChar, "name"}, {fdb.VariantReal, "x"}, {fdb.VariantReal, "y"}, {fdb.VariantReal, "z"}, {fdb.VariantI64, "type"}}},
		},
		Rows: map[string][]fdb.Row{
			"Accounts": {
				{entry(fdb.VariantU32, uint32(0)), entry(fdb.VariantNVarChar, "Alice"), entry(fdb.VariantU32, uint32(20)), entry(fdb.VariantBool, true)},
				{entry(fdb.VariantU32, uint32(1)), entry(fdb.VariantNVarChar, "Bob"), entry(fdb.VariantU32, uint32(21)), entry(fdb.VariantBool, true)},
				{entry(fdb.VariantU32, uint32(2)), entry(fdb.VariantNVarChar, "Charlie"), entry(fdb.VariantU32, uint32(22)), entry(fdb.VariantBool, false)},
				{entry(fdb.VariantU32, uint32(3)), entry(fdb.VariantNVarChar, "David"), entry(fdb.VariantU32, uint32(23)), entry(fdb.VariantBool, true)},
				{entry(fdb.VariantU32, uint32(4)), entry(fdb.VariantNVarChar, "Eve"), entry(fdb.VariantU32, uint32(24)), entry(fdb.VariantBool, true)},
			},
			"Skills": {
				{entry(fdb.VariantU32, uint32(0)), entry(fdb.VariantText, "JUMP"), entry(fdb.VariantU64, uint64(10))},
				{entry(fdb.VariantU32, uint32(0)), entry(fdb.VariantText, "KICK"), entry(fdb.VariantU64, uint64(5))},
				{entry(fdb.VariantU32, uint32(0)), entry(fdb.VariantText, "PUNCH"), entry(fdb.VariantU64, uint64(20))},
				{entry(fdb.VariantU32, uint32(1)), entry(fdb.VariantText, "JUMP"), entry(fdb.VariantU64, uint64(15))},
				{entry(fdb.VariantU32, uint32(3)), entry(fdb.VariantText, "JUMP"), entry(fdb.VariantU64, uint64(20))},
				{entry(fdb.VariantU32, uint32(3)), entry(fdb.VariantText, "PUNCH"), entry(fdb.VariantU64, uint64(50))},
			},
			"Version": {
				{entry(fdb.VariantI32, int32(1)), entry(fdb.VariantI32, int32(48)), entry(fdb.VariantI32, int32(13))},
			},
			"NPCs": {
				{entry(fdb.VariantNVarChar, "Doctor Overbuild"), entry(fdb.VariantReal, float32(84.76535107832655)), entry(fdb.VariantReal, float32(-56.22811218552046)), entry(fdb.VariantReal, float32(-76.39466766529857)), entry(fdb.VariantI64, int64(10))},
				{entry(fdb.VariantNVarChar, "Hael Storm"), entry(fdb.VariantReal, float32(29.52846880879494)), entry(fdb.VariantReal, float32(9.158960041088449)), entry(fdb.VariantReal, float32(37.72298090662753)), entry(fdb.VariantI64, int64(15))},
				{entry(fdb.VariantNVarChar, "Duke Exeter"), entry(fdb.VariantReal, float32(74.72681257230235)), entry(fdb.VariantReal, float32(23.370179324588918)), entry(fdb.VariantReal, float32(-52.329307984643016)), entry(fdb.VariantI64, int64(20))},
				{entry(fdb.VariantNVarChar, "Vanda Darkflame"), entry(fdb.VariantReal, float32(86.26251744412366)), entry(fdb.VariantReal, float32(90.3301364318929)), entry(fdb.VariantReal, float32(-27.163601074604088)), entry(fdb.VariantI64, int64(25))},
			},
		},
	}))
}

func createTable(name string, tables []*fdb.Table, rows map[string][]fdb.Row) error {
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	builder := fdb.NewBuilder(file, tables)
	if err := builder.Flush(func(tableName string) iter.Seq2[fdb.Row, error] {
		return func(yield func(fdb.Row, error) bool) {
			for _, row := range rows[tableName] {
				if !yield(row, nil) {
					return
				}
			}
		}
	}); err != nil {
		return err
	}

	return nil
}

func randomString() string {
	const chars = "abcdefghijklmnopqrstuvwxyz"

	s := make([]byte, rand.Intn(5)+5)
	for i := range s {
		s[i] = chars[i]
	}
	return string(s)
}

func createRow(columns []*fdb.Column) fdb.Row {
	row := make([]fdb.Entry, 0, len(columns))

	for i := range len(columns) {
		col := columns[i]

		switch col.Variant {
		case fdb.VariantI32:
			row = append(row, entry(col.Variant, rand.Int31()))
		case fdb.VariantU32:
			row = append(row, entry(col.Variant, rand.Uint32()))
		case fdb.VariantReal:
			row = append(row, entry(col.Variant, rand.Float32()))
		case fdb.VariantNVarChar, fdb.VariantText:
			row = append(row, entry(col.Variant, randomString()))
		case fdb.VariantBool:
			row = append(row, entry(col.Variant, rand.Intn(2) == 0))
		case fdb.VariantI64:
			row = append(row, entry(col.Variant, rand.Int63()))
		case fdb.VariantU64:
			row = append(row, entry(col.Variant, rand.Uint64()))
		default:
			panic(fmt.Errorf("cannot create entry for variant %v", col.Variant))
		}
	}

	return row
}

func testWrite(dir string, tables []*fdb.Table) func(*testing.T) {
	return func(t *testing.T) {
		db := TestDb{
			Tables: tables,
			Rows:   make(map[string][]fdb.Row),
		}
		for _, table := range tables {
			rows := make([]fdb.Row, rand.Intn(10)+10)
			for i := range rows {
				rows[i] = createRow(table.Columns)
			}

			db.Rows[table.Name] = rows
		}

		fdbName := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+".fdb")
		if err := createTable(fdbName, db.Tables, db.Rows); err != nil {
			t.Fatal(err)
		}
		t.Logf("created FDB file: %s", fdbName)

		reader, err := fdb.OpenReader(fdbName)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		checkReader(t, db, reader)
	}
}

func TestWrite(t *testing.T) {
	dir, err := os.MkdirTemp("testdata", "fdb*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	t.Run("write", testWrite(dir, []*fdb.Table{
		{Name: "Table1", Columns: []*fdb.Column{{fdb.VariantU32, "id"}, {fdb.VariantReal, "someData"}, {fdb.VariantBool, "active"}}},
		{Name: "Table2", Columns: []*fdb.Column{{fdb.VariantNVarChar, "id"}, {fdb.VariantU64, "unsigned"}, {fdb.VariantText, "longText"}, {fdb.VariantI64, "signed"}}},
		{Name: "Table3", Columns: []*fdb.Column{{fdb.VariantBool, "boolId"}, {fdb.VariantI32, "index"}}},
	}))
}
