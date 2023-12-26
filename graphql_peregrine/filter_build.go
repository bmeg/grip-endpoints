package main

import (
	"github.com/bmeg/grip/gripql"
	"github.com/bmeg/grip/log"
)

type FilterBuilder struct {
	filter map[string]any
}

func NewFilterBuilder(i map[string]any) *FilterBuilder {
	return &FilterBuilder{i}
}

func fieldMap(s string) string {
	if s == "id" {
		return "_gid"
	}
	return s
}

func (fb *FilterBuilder) ExtendGrip(q *gripql.Query, filterSelfName string) (*gripql.Query, error) {
	// isFilter filters out a top level "AND" that seems to be consistant across all queries in the exploration page

	for _, array_filter := range fb.filter {
		if map_array_filter, ok := array_filter.(map[string]any); ok {
			for filter_key, arr_filter_values := range map_array_filter {
				filter_key = fieldMap(filter_key)
				if filter_values, ok := arr_filter_values.([]any); ok {
					if len(filter_values) == 1 {
						q = q.Has(gripql.Within(filter_key, filter_values[0]))

					} else if len(filter_values) > 1 {
						final_expr := gripql.Or(gripql.Within(filter_key, filter_values[0]), gripql.Within(filter_key, filter_values[1]))
						for i := 2; i < len(filter_values); i++ {
							final_expr = gripql.Or(final_expr, gripql.Within(filter_key, filter_values[i]))
						}
						q = q.Has(final_expr)
					} else {
						log.Error("Error state checkbox filter not populated but list was created")
					}

				}

			}

		}
	}

	log.Infof("Filter Query %s", q.String())
	return q, nil
}
