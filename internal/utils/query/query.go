package query

import (
	"fmt"
	"regexp"
	"strings"
)

type Direction string

const (
	Asc  Direction = "ASC"
	Desc Direction = "DESC"
)

type Operator string

const (
	OpEqual        Operator = "="
	OpNotEqual     Operator = "!="
	OpGreater      Operator = ">"
	OpGreaterEqual Operator = ">="
	OpLess         Operator = "<"
	OpLessEqual    Operator = "<="
	OpLike         Operator = "LIKE"
)

var validOperators = map[Operator]struct{}{
	OpEqual: {}, OpNotEqual: {}, OpGreater: {}, OpGreaterEqual: {}, OpLess: {}, OpLessEqual: {}, OpLike: {},
}

type Filter struct {
	Column   string
	Operator Operator
	Value    any
}

type Sort struct {
	Column string
	Dir    Direction
}

type Page struct {
	Number int
	Size   int
}

type Query struct {
	Filters       []Filter
	Search        string
	SearchColumns []string
	Sort          []Sort
	Page          Page
}

const (
	DefaultPageSize = 50
	MaxPageSize     = 1000
)

type Built struct {
	SelectSQL  string
	SelectArgs []any
	CountSQL   string
	CountArgs  []any
}

type Builder struct {
	table         string
	allowed       map[string]struct{}
	defaultSort   []Sort
	selectColumns []string
}

var tableNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func NewBuilder(table string, allowedColumns ...string) *Builder {
	allowed := make(map[string]struct{}, len(allowedColumns))
	for _, c := range allowedColumns {
		allowed[c] = struct{}{}
	}
	return &Builder{table: table, allowed: allowed}
}

func (b *Builder) SelectColumns(columns ...string) *Builder {
	b.selectColumns = append(b.selectColumns, columns...)
	return b
}

func (b *Builder) DefaultSort(column string, dir Direction) *Builder {
	b.defaultSort = append(b.defaultSort, Sort{Column: column, Dir: dir})
	return b
}

func (b *Builder) Build(q Query) (Built, error) {
	if err := b.validate(q); err != nil {
		return Built{}, err
	}

	var where []string
	var args []any

	for _, f := range q.Filters {
		where = append(where, f.Column+" "+string(f.Operator)+" ?")
		args = append(args, f.Value)
	}

	if q.Search != "" {
		var parts []string
		term := "%" + escapeLike(q.Search) + "%"
		for _, col := range q.SearchColumns {
			parts = append(parts, col+" LIKE ? ESCAPE '\\'")
			args = append(args, term)
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var order []string
	sort := q.Sort
	if len(sort) == 0 {
		sort = b.defaultSort
	}
	for _, s := range sort {
		order = append(order, s.Column+" "+string(s.Dir))
	}
	orderSQL := ""
	if len(order) > 0 {
		orderSQL = " ORDER BY " + strings.Join(order, ", ")
	}

	size, number := normalizePage(q.Page)
	selectList := "*"
	if len(b.selectColumns) > 0 {
		selectList = strings.Join(b.selectColumns, ", ")
	}
	selectSQL := "SELECT " + selectList + " FROM " + b.table + whereSQL + orderSQL + " LIMIT ? OFFSET ?"
	selectArgs := append(args, size, (number-1)*size)

	countSQL := "SELECT COUNT(*) FROM " + b.table + whereSQL

	return Built{
		SelectSQL:  selectSQL,
		SelectArgs: selectArgs,
		CountSQL:   countSQL,
		CountArgs:  args,
	}, nil
}

func (b *Builder) validate(q Query) error {
	if !tableNameRe.MatchString(b.table) {
		return fmt.Errorf("query: invalid table name %q", b.table)
	}

	for _, f := range q.Filters {
		if _, ok := b.allowed[f.Column]; !ok {
			return fmt.Errorf("query: column %q not allowed", f.Column)
		}
		if _, ok := validOperators[f.Operator]; !ok {
			return fmt.Errorf("query: operator %q not allowed", f.Operator)
		}
	}

	for _, col := range q.SearchColumns {
		if _, ok := b.allowed[col]; !ok {
			return fmt.Errorf("query: search column %q not allowed", col)
		}
	}
	if q.Search != "" && len(q.SearchColumns) == 0 {
		return fmt.Errorf("query: search term requires at least one search column")
	}

	sorts := q.Sort
	if len(sorts) == 0 {
		sorts = b.defaultSort
	}
	for _, s := range sorts {
		if _, ok := b.allowed[s.Column]; !ok {
			return fmt.Errorf("query: sort column %q not allowed", s.Column)
		}
		if s.Dir != Asc && s.Dir != Desc {
			return fmt.Errorf("query: sort direction %q not allowed", s.Dir)
		}
	}

	return nil
}

func normalizePage(p Page) (size, number int) {
	size = p.Size
	if size <= 0 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	number = p.Number
	if number <= 0 {
		number = 1
	}
	return size, number
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
