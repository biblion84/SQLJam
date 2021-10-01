package app

import (
	"fmt"
	"strings"
)

type SqlSource interface {
	SourceToSql(indent int) string
	SourceAlias() string
}

// A context for node generation recursion.
// Eventually, we can no longer add onto this query. Thus,
// we continue recursive generation with a new Source context object.
// Thus this is basically a recursive tree
type QueryContext struct {
	Alias  string
	Source SqlSource // or NodeGenContext

	// Picked columns and aggregates are mutually exclusive.
	Cols      []GenColumn
	Aggregate *GenAggregate

	Combines         []GenCombine
	Joins            []GenJoin
	WhereConditions  []string
	HavingConditions []string
	Orders           []GenOrder
}

var _ SqlSource = &QueryContext{}

// NewQueryContext Creates an empty query context.
func NewQueryContext() *QueryContext {
	return &QueryContext{}
}

// NewQueryContextFromNode Creates a query context and fills it with data from a given node.
// For example, calling this with a Table/Filter will result in this
// structure:
//
//	context:
//	  source: table
//    filters: filters
func NewQueryContextFromNode(n *Node) *QueryContext {
	return NewQueryContext().CreateQuery(n)
}

// WrapQueryContext Wraps the given query context in a new, empty context. Use when you
// want to break something...into a subquery.
func WrapQueryContext(ctx *QueryContext) *QueryContext {
	return &QueryContext{
		Source: ctx,
	}
}

func (ctx *QueryContext) SourceAlias() string {
	return ctx.Alias
}

type GenColumn struct {
	Col   string
	Alias string
}

type GenAggregate struct {
	GroupByCols []string
	Aggs        []GenAggregateEntry
}

type GenAggregateEntry struct {
	Type  AggregateType
	Col   string
	Alias string
}

type GenCombine struct {
	Context *QueryContext
	Type    CombineType
}

type GenOrder struct {
	Col        string
	Descending bool
}

type GenJoin struct {
	Source    SqlSource
	Condition string
	Type      JoinType
}

func Indented(s string, amount int) string {
	return strings.Repeat("\t", amount) + s
}

// SourceToSql Turns a context tree into an SQL statement string
func (ctx *QueryContext) SourceToSql(indent int) string {
	var sql string

	if len(ctx.Combines) > 0 {
		sql += ctx.Source.SourceToSql(indent)
		for _, gc := range ctx.Combines {
			used := ""
			switch gc.Type {
			case Union:
				used = "UNION"
			case Intersect:
				used = "INTERSECT"
			case Except:
				used = "EXCEPT"
			case UnionAll:
				used = "UNION ALL"
			}
			sql += "\n" + Indented(used, indent)
			sql += "\n" + gc.Context.SourceToSql(indent) + "\n"
		}
	} else {
		sql += Indented("SELECT ", indent)

		if len(ctx.Cols) > 0 && ctx.Aggregate != nil {
			return Indented("Error: Pick columns and aggregate on the same context", indent)
		}

		if ctx.Aggregate != nil {
			var colStrings []string
			for _, gbCol := range ctx.Aggregate.GroupByCols {
				colStrings = append(colStrings, gbCol)
			}
			for _, agg := range ctx.Aggregate.Aggs {
				op := "ERROR("
				switch agg.Type {
				case Avg:
					op = "AVG("
				case Max:
					op = "MAX("
				case Min:
					op = "MIN("
				case Sum:
					op = "SUM("
				case Count:
					op = "COUNT("
				case CountDistinct:
					op = "COUNT(DISTINCT "
				}

				aliasStr := ""
				if agg.Alias != "" {
					aliasStr = fmt.Sprintf(" AS %s", agg.Alias)
				}

				colStrings = append(colStrings, fmt.Sprintf("%s%s)%s", op, agg.Col, aliasStr))
			}
			sql += strings.Join(colStrings, ", ")
		} else if len(ctx.Cols) == 0 {
			sql += "*"
		} else {
			colStrings := make([]string, len(ctx.Cols))
			for i, col := range ctx.Cols {
				aliasStr := ""
				if col.Alias != "" {
					aliasStr = fmt.Sprintf(" AS %s", col.Alias)
				}
				colStrings[i] = fmt.Sprintf("%s%s", col.Col, aliasStr)
			}
			sql += strings.Join(colStrings, ", ")
		}

		if ctx.Source == nil {
			return Indented("Error: No SQL Source", indent)
		} else {
			sql += "\n" + Indented("", indent)
			switch ctx.Source.(type) {
			case *Table:
				sql += fmt.Sprintf("FROM %s", ctx.Source.SourceToSql(0))
			default:
				sql += fmt.Sprintf("FROM (\n%s", ctx.Source.SourceToSql(indent+1))
				sql += "\n" + Indented(")", indent)
			}

			sql += " AS a"
		}

		for _, join := range ctx.Joins {
			sql += "\n" + Indented(join.Type.String(), indent) + " "
			switch join.Source.(type) {
			case *Table:
				sql += join.Source.SourceToSql(indent)
			default:
				sql += "(\n" + join.Source.SourceToSql(indent+1)
				sql += "\n" + Indented(")", indent)
			}
			sql += fmt.Sprintf(" AS %s", join.Source.SourceAlias())
			if join.Condition != "" {
				sql += fmt.Sprintf(" ON %s", join.Condition)
			}
		}

		if len(ctx.WhereConditions) > 0 && ctx.WhereConditions[0] != "" {
			sql += "\n" + Indented("WHERE ", indent)
			sql += strings.Join(ctx.WhereConditions, " AND ")
		}
	}

	if ctx.Aggregate != nil && len(ctx.Aggregate.GroupByCols) > 0 {
		sql += "\n" + Indented("GROUP BY ", indent)
		sql += strings.Join(ctx.Aggregate.GroupByCols, ", ")
	}

	if len(ctx.HavingConditions) > 0 {
		sql += "\n" + Indented("HAVING ", indent)
		sql += strings.Join(ctx.HavingConditions, " AND ")
	}

	if len(ctx.Orders) > 0 {
		sql += Indented("\nORDER BY ", indent)
		var orderStrings []string
		for _, order := range ctx.Orders {
			direction := ""
			if order.Descending {
				direction = " DESC"
			}
			orderStrings = append(orderStrings, fmt.Sprintf("%s%s", order.Col, direction))
		}
		sql += strings.Join(orderStrings, ", ")
	}

	return sql
}

// CreateQuery Turns a node into a recursive context tree for SQL generation
func (ctx *QueryContext) CreateQuery(n *Node) *QueryContext {
	if n == nil {
		return ctx
	}

	switch d := n.Data.(type) {
	case *Table:
		ctx.Source = d
		ctx.Alias = d.Alias
	case *PickColumns:
		ctx = ctx.CreateQuery(n.Inputs[0])
		if len(ctx.Cols) > 0 || ctx.Aggregate != nil {
			ctx = WrapQueryContext(ctx)
		}

		for _, e := range d.Entries {
			ctx.Cols = append(ctx.Cols, GenColumn{
				Col:   e.Col,
				Alias: e.Alias,
			})
		}
		// ctx.Alias = d.Alias // TODO: ???????
	case *Filter:
		ctx = ctx.CreateQuery(n.Inputs[0])
		if len(ctx.Cols) > 0 {
			ctx = WrapQueryContext(ctx)
		}

		if ctx.Aggregate != nil {
			ctx.HavingConditions = append(ctx.HavingConditions, d.Conditions)
		} else {
			ctx.WhereConditions = append(ctx.WhereConditions, d.Conditions)
		}
	case *Order:
		ctx = ctx.CreateQuery(n.Inputs[0])
		if len(ctx.Orders) > 0 {
			ctx = WrapQueryContext(ctx)
		}

		for _, col := range d.Cols {
			ctx.Orders = append(ctx.Orders, GenOrder{
				Col:        col.Col,
				Descending: col.Descending,
			})
		}
	case *CombineRows:
		firstCtx := NewQueryContextFromNode(n.Inputs[0])
		firstCtx.Orders = nil // anything involved in Combine Rows can't use ORDER BY

		ctx = WrapQueryContext(firstCtx)

		// All other inputs get thrown into a new recursive context
		for _, input := range n.Inputs[1:] {
			if input != nil {
				newCtx := NewQueryContextFromNode(input)
				newCtx.Orders = nil
				ctx.Combines = append(ctx.Combines, GenCombine{
					Context: newCtx,
					Type:    d.CombinationType,
				})
			}
		}
	case *Join:
		if n.Inputs[0] != nil {
			if table, ok := n.Inputs[0].Data.(*Table); ok {
				ctx = NewQueryContext()
				ctx.Source = table
			} else {
				firstCtx := NewQueryContextFromNode(n.Inputs[0])
				ctx = WrapQueryContext(firstCtx)
			}

			// All other inputs get thrown into a new recursive context
			for i, input := range n.Inputs[1:] {
				if input != nil {
					aliasChar := string(rune('b' + i))
					cond := d.Conditions[i]
					var source SqlSource
					if table, ok := input.Data.(*Table); ok {
						table.Alias = aliasChar
						source = table
					} else {
						newQuery := NewQueryContextFromNode(input)
						newQuery.Alias = aliasChar
						source = newQuery
					}
					ctx.Joins = append(ctx.Joins, GenJoin{
						Source:    source,
						Condition: cond.Condition,
						Type:      cond.Type(),
					})
				}
			}
		}
	case *Aggregate:
		ctx = ctx.CreateQuery(n.Inputs[0])
		if len(ctx.Cols) > 0 || ctx.Aggregate != nil {
			ctx = WrapQueryContext(ctx)
		}

		groupByCols := make([]string, len(d.GroupBys))
		for i, gb := range d.GroupBys {
			groupByCols[i] = gb.Col
		}

		aggs := make([]GenAggregateEntry, len(d.Aggregates))
		for i, agg := range d.Aggregates {
			aggs[i] = GenAggregateEntry{
				Type:  agg.Type,
				Col:   agg.Col,
				Alias: agg.Alias,
			}
		}

		ctx.Aggregate = &GenAggregate{
			GroupByCols: groupByCols,
			Aggs:        aggs,
		}
	}

	return ctx
}

func (ctx *QueryContext) RecursiveGenerateInputs(n *Node) *QueryContext {
	for _, value := range n.Inputs {
		if value != nil {
			ctx.CreateQuery(value)
		}
	}
	return ctx
}

func (n *Node) GenerateSql() string {
	return NewQueryContextFromNode(n).SourceToSql(0)
}
