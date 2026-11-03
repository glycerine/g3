package cmd_context

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

func joinClose(err error, closeErr error) error { return errors.Join(err, closeErr) }

func (p *wpaParser) readMaps() error {
	files, err := p.files("map")
	if err != nil {
		return err
	}
	for _, path := range files {
		entry := &wpaSort{name: wpaBase(path), numbers: make(map[uint64]bool)}
		p.sorts[entry.name] = entry
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			number, name, present, parseErr := parseWPAMapLine(scanner.Text())
			if parseErr != nil {
				file.Close()
				return fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			if !present {
				continue
			}
			entry.numbers[number] = true
			// Source num2sym::insert_if_not_there2 keeps the first name when
			// independent sort maps reuse a numeric symbol. A different later
			// spelling is warning-only and must not reject the directory.
			if _, found := p.mapNames[number]; !found {
				p.mapNames[number] = name
			}
		}
		err = scanner.Err()
		err = joinClose(err, file.Close())
		if err != nil {
			return err
		}
	}
	return nil
}
