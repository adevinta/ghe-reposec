// Copyright 2025 Adevinta

// Package output provides functions to write the output of the ghe-reposec tool.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/adevinta/ghe-reposec/internal/lava"
)

var (
	// ErrUnsupportedFormat is returned when the output format is not supported.
	ErrUnsupportedFormat = fmt.Errorf("unsupported format")

	// ErrOutputFileRequired is returned when the output file is not provided.
	ErrOutputFileRequired = fmt.Errorf("output file is required and was not provided")
)

// Write writes the output of the ghe-reposec tool.
func Write(format, file string, summary []lava.Summary) error {
	if file == "" {
		return ErrOutputFileRequired
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	switch strings.ToLower(format) {
	case "csv":
		writer := csv.NewWriter(f)
		defer writer.Flush()

		err := writer.Write(
			[]string{
				"repository",
				"control_in_place",
				"number_of_controls",
				"controls",
				"error",
			},
		)
		if err != nil {
			return err
		}
		for _, s := range summary {
			err := writer.Write(
				[]string{
					s.Repository,
					strconv.FormatBool(s.ControlInPlace),
					strconv.Itoa(s.NumberOfControls),
					strings.Join(s.Controls, "#"),
					s.Error,
				},
			)
			if err != nil {
				return err
			}
		}
	case "json":
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "  ")
		err := encoder.Encode(summary)
		if err != nil {
			return err
		}
	default:
		return ErrUnsupportedFormat
	}

	return nil
}
