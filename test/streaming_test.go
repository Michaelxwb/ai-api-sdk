package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

func collectChunks(ch <-chan streaming.StreamChunk) []streaming.StreamChunk {
	chunks := make([]streaming.StreamChunk, 0, 8)
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	return chunks
}

func TestSSEParser_ParseDataAndDone(t *testing.T) {
	payload := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n" +
		"data: [DONE]\n\n"

	buf := bytes.NewBufferString(payload)
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(buf),
	}

	parser := &streaming.SSEParser{Config: streaming.StreamConfig{DoneMarker: "[DONE]"}}
	extractor := streaming.MakeJSONPathExtractor("choices.0.delta.content", "", "", "")
	ch, err := parser.Parse(context.Background(), resp, extractor)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	chunks := collectChunks(ch)
	if len(chunks) != 3 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
	if chunks[0].Text != "hello " || chunks[0].Done || chunks[0].Error != nil {
		t.Fatalf("unexpected first chunk: %+v", chunks[0])
	}
	if chunks[1].Text != "world" || chunks[1].Done || chunks[1].Error != nil {
		t.Fatalf("unexpected second chunk: %+v", chunks[1])
	}
	if !chunks[2].Done || string(chunks[2].Raw) != "[DONE]" {
		t.Fatalf("unexpected done chunk: %+v", chunks[2])
	}
}

func TestNDJSONParser_Parse(t *testing.T) {
	payload := "" +
		"{\"message\":{\"content\":\"a\"},\"done\":false}\n" +
		"{\"message\":{\"content\":\"b\"},\"done\":true}\n"

	buf := bytes.NewBufferString(payload)
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(buf),
	}

	parser := &streaming.NDJSONParser{Config: streaming.StreamConfig{}}
	extractor := streaming.MakeJSONPathExtractor("message.content", "done", "true", "")
	ch, err := parser.Parse(context.Background(), resp, extractor)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	chunks := collectChunks(ch)
	if len(chunks) != 2 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
	if chunks[0].Text != "a" || chunks[0].Done {
		t.Fatalf("unexpected first chunk: %+v", chunks[0])
	}
	if chunks[1].Text != "b" || !chunks[1].Done {
		t.Fatalf("unexpected second chunk: %+v", chunks[1])
	}
}

func TestExtractJSONPath(t *testing.T) {
	data := []byte(`{"choices":[{"delta":{"content":"hi"}}]}`)
	val, err := streaming.ExtractJSONPath(data, "choices.0.delta.content")
	if err != nil || val != "hi" {
		t.Fatalf("unexpected extraction: %q, err=%v", val, err)
	}
	val, err = streaming.ExtractJSONPath(data, "choices[0].delta.content")
	if err != nil || val != "hi" {
		t.Fatalf("unexpected extraction: %q, err=%v", val, err)
	}
	if _, err := streaming.ExtractJSONPath(data, ""); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestSSEParser_ErrorChunk(t *testing.T) {
	payload := "data: {\"bad\":1}\n\n"
	buf := bytes.NewBufferString(payload)
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(buf),
	}

	parser := &streaming.SSEParser{Config: streaming.StreamConfig{}}
	extractor := func(event string, data []byte) (string, bool, error) {
		return "", false, io.ErrUnexpectedEOF
	}
	ch, err := parser.Parse(context.Background(), resp, extractor)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	chunks := collectChunks(ch)
	if len(chunks) == 0 || chunks[0].Error == nil {
		t.Fatalf("expected error chunk, got: %+v", chunks)
	}
}

func TestSSEParser_EOFHandling(t *testing.T) {
	payload := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}"
	buf := bytes.NewBufferString(payload)
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       io.NopCloser(buf),
	}

	parser := &streaming.SSEParser{Config: streaming.StreamConfig{}}
	extractor := streaming.MakeJSONPathExtractor("choices.0.delta.content", "", "", "")
	ch, err := parser.Parse(context.Background(), resp, extractor)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	chunks := collectChunks(ch)
	if len(chunks) != 1 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
	if chunks[0].Text != "hi" {
		t.Fatalf("unexpected chunk: %+v", chunks[0])
	}
}
