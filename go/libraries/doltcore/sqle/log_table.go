// Copyright 2019 Liquidata, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqle

import (
	"io"

	"github.com/liquidata-inc/go-mysql-server/sql"

	"github.com/liquidata-inc/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/env/actions"
)

var _ sql.Table = (*LogTable)(nil)

// LogTable is a sql.Table implementation that implements a system table which shows the dolt commit log
type LogTable struct {
	dbName string
	ddb    *doltdb.DoltDB
}

// NewLogTable creates a LogTable
func NewLogTable(ctx *sql.Context, dbName string) (*LogTable, error) {
	ddb, ok := DSessFromSess(ctx.Session).GetDoltDB(dbName)

	if !ok {
		return nil, sql.ErrDatabaseNotFound.New(dbName)
	}

	return &LogTable{dbName: dbName, ddb: ddb}, nil
}

// Name is a sql.Table interface function which returns the name of the table which is defined by the constant
// LogTableName
func (dt *LogTable) Name() string {
	return doltdb.LogTableName
}

// String is a sql.Table interface function which returns the name of the table which is defined by the constant
// LogTableName
func (dt *LogTable) String() string {
	return doltdb.LogTableName
}

// Schema is a sql.Table interface function that gets the sql.Schema of the log system table.
func (dt *LogTable) Schema() sql.Schema {
	return []*sql.Column{
		{Name: "commit_hash", Type: sql.Text, Source: doltdb.LogTableName, PrimaryKey: true},
		{Name: "committer", Type: sql.Text, Source: doltdb.LogTableName, PrimaryKey: false},
		{Name: "email", Type: sql.Text, Source: doltdb.LogTableName, PrimaryKey: false},
		{Name: "date", Type: sql.Datetime, Source: doltdb.LogTableName, PrimaryKey: false},
		{Name: "message", Type: sql.Text, Source: doltdb.LogTableName, PrimaryKey: false},
	}
}

// Partitions is a sql.Table interface function that returns a partition of the data.  Currently the data is unpartitioned.
func (dt *LogTable) Partitions(*sql.Context) (sql.PartitionIter, error) {
	return &doltTablePartitionIter{}, nil
}

// PartitionRows is a sql.Table interface function that gets a row iterator for a partition
func (dt *LogTable) PartitionRows(sqlCtx *sql.Context, part sql.Partition) (sql.RowIter, error) {
	return NewLogItr(sqlCtx, dt.dbName, dt.ddb)
}

// LogItr is a sql.RowItr implementation which iterates over each commit as if it's a row in the table.
type LogItr struct {
	commits []*doltdb.Commit
	idx     int
}

// NewLogItr creates a LogItr from the current environment.
func NewLogItr(sqlCtx *sql.Context, dbName string, ddb *doltdb.DoltDB) (*LogItr, error) {
	sess := DSessFromSess(sqlCtx.Session)
	commit, err := sess.GetParentCommit(sqlCtx, dbName)

	if err != nil {
		return nil, err
	}

	commits, err := actions.TimeSortedCommits(sqlCtx, ddb, commit, -1)

	if err != nil {
		return nil, err
	}

	return &LogItr{commits, 0}, nil
}

// Next retrieves the next row. It will return io.EOF if it's the last row.
// After retrieving the last row, Close will be automatically closed.
func (itr *LogItr) Next() (sql.Row, error) {
	if itr.idx >= len(itr.commits) {
		return nil, io.EOF
	}

	defer func() {
		itr.idx++
	}()

	cm := itr.commits[itr.idx]
	meta, err := cm.GetCommitMeta()

	if err != nil {
		return nil, err
	}

	h, err := cm.HashOf()

	if err != nil {
		return nil, err
	}

	return sql.NewRow(h.String(), meta.Name, meta.Email, meta.Time(), meta.Description), nil
}

// Close closes the iterator.
func (itr *LogItr) Close() error {
	return nil
}
