package main

import (
	"bytes"
	"fmt"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/json"
	"math/rand"
	"net/http"
	"strconv"
)

func (a *Anicord) handleVerify(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, a.oauth2.GenerateAuthorizationURL(baseURL+"/discord", discord.PermissionsNone, 0, false, discord.OAuth2ScopeIdentify, discord.OAuth2ScopeRoleConnectionsWrite), http.StatusTemporaryRedirect)
}

func (a *Anicord) handleDiscord(w http.ResponseWriter, r *http.Request) {
	var (
		query = r.URL.Query()
		code  = query.Get("code")
		state = query.Get("state")
	)
	if code == "" || state == "" {
		writeError(w, "invalid request", nil)
	}

	identifier := randStr(32)
	_, err := a.oauth2.StartSession(code, state, identifier)
	if err != nil {
		writeError(w, "error while starting session", err)
		return
	}

	http.Redirect(w, r, a.anilist.AuthCodeURL(identifier), http.StatusTemporaryRedirect)
}

func (a *Anicord) handleAnilist(w http.ResponseWriter, r *http.Request) {
	var (
		query = r.URL.Query()
		code  = query.Get("code")
		state = query.Get("state")
	)
	if code == "" || state == "" {
		writeError(w, "invalid request", nil)
	}

	session := a.oauth2.SessionController().GetSession(state)
	if session == nil {
		writeError(w, "invalid session", nil)
		return
	}

	token, err := a.anilist.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, "error while exchanging token", err)
		return
	}
	httpClient := a.anilist.Client(r.Context(), token)

	body := map[string]any{
		"query": `
			query ($id: Int) {
				Viewer {
					id
					name
					statistics {
						anime {
							count
						}
						manga {
							count
						}
					}
				}
			}`,
		"variables": map[string]any{
			"userId": token.Extra("user_id"),
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		writeError(w, "error while marshaling gql body", err)
		return
	}

	rq, err := http.NewRequest(http.MethodPost, "https://graphql.anilist.co", bytes.NewBuffer(data))
	if err != nil {
		writeError(w, "error while creating request", err)
		return
	}

	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Accept", "application/json")

	rs, err := httpClient.Do(rq)
	if err != nil {
		writeError(w, "error while executing request", err)
		return
	} else if rs.StatusCode != http.StatusOK {
		writeError(w, "invalid status code", fmt.Errorf("status code: %d", rs.StatusCode))
		return
	}
	defer rs.Body.Close()

	var v anilistResponse
	if err = json.NewDecoder(rs.Body).Decode(&v); err != nil {
		writeError(w, "error while decoding response", err)
		return
	}

	if _, err = a.oauth2.UpdateApplicationRoleConnection(session, a.client.ApplicationID(), discord.ApplicationRoleConnectionUpdate{
		PlatformName:     json.Ptr("Anilist"),
		PlatformUsername: &v.Data.Viewer.Name,
		Metadata: &map[string]string{
			"anime_count": strconv.Itoa(v.Data.Viewer.Statistics.Anime.Count),
			"manga_count": strconv.Itoa(v.Data.Viewer.Statistics.Manga.Count),
		},
	}); err != nil {
		writeError(w, "error while updating application role connection", err)
		return
	}

	_, _ = w.Write([]byte("success"))
}

func writeError(w http.ResponseWriter, text string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(text + ": " + err.Error()))
}

func randStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
