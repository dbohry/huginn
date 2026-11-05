package server

import (
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/dbohry/huginn/internal/store"
)

const (
	maxBlockedWordRunes = 60
	maxBlockedWords     = 200
)

type blockedWordJSON struct {
	ID   int64  `json:"id"`
	Word string `json:"word"`
}

func (s *server) listBlockedWords(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	words, err := s.store.ListBlockedWords(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	out := make([]blockedWordJSON, len(words))
	for i, b := range words {
		out[i] = blockedWordJSON{ID: b.ID, Word: b.Word}
	}
	writeJSON(w, http.StatusOK, map[string][]blockedWordJSON{"words": out})
}

func (s *server) addBlockedWord(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	var in struct {
		Word string `json:"word"`
	}
	if !decode(w, r, &in) {
		return
	}
	word := store.NormalizeBlockedWord(in.Word)
	switch {
	case word == "":
		writeError(w, http.StatusBadRequest, "Enter a word to block.")
		return
	case utf8.RuneCountInString(word) > maxBlockedWordRunes:
		writeError(w, http.StatusBadRequest, "A blocked word can have at most 60 characters.")
		return
	}
	existing, err := s.store.ListBlockedWords(r.Context(), user.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	if len(existing) >= maxBlockedWords {
		writeError(w, http.StatusBadRequest, "You can block at most 200 words.")
		return
	}
	b, err := s.store.AddBlockedWord(r.Context(), user.ID, word)
	switch {
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "That word is already blocked.")
	case err != nil:
		serverError(w, err)
	default:
		writeJSON(w, http.StatusCreated, map[string]blockedWordJSON{"word": {ID: b.ID, Word: b.Word}})
	}
}

func (s *server) deleteBlockedWord(w http.ResponseWriter, r *http.Request, user store.User, _ store.Session) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "Blocked word not found.")
		return
	}
	switch err := s.store.DeleteBlockedWord(r.Context(), user.ID, id); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Blocked word not found.")
	case err != nil:
		serverError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
