// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package es

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/elastic/go-elasticsearch/v8"
)

// doSearch executes a search request with the provided body, index, size, and sort.
func doSearch(ctx context.Context, client *elasticsearch.Client, index string, queryJSON []byte, size int, sort string) (io.ReadCloser, string, bool, error) {
	res, err := client.Search(
		client.Search.WithContext(ctx),
		client.Search.WithIndex(index),
		client.Search.WithBody(bytes.NewReader(queryJSON)),
		client.Search.WithSize(size),
		client.Search.WithSort(sort),
	)
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to execute search: %w", err)
	}
	return res.Body, res.Status(), res.IsError(), nil
}
