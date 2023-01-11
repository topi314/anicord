package main

import (
	"math/rand"
	"net/http"

	"github.com/disgoorg/disgo/discord"
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
		return
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
		return
	}

	session := a.oauth2.SessionController().GetSession(state)
	if session == nil {
		writeError(w, "invalid session", nil)
		return
	}

	discordUser, err := a.oauth2.GetUser(session)
	if err != nil {
		writeError(w, "error while getting user", err)
		return
	}

	token, err := a.anilist.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, "error while exchanging token", err)
		return
	}

	user := User{
		DiscordID:           discordUser.ID,
		DiscordAccessToken:  session.AccessToken(),
		DiscordRefreshToken: session.RefreshToken(),
		DiscordExpiry:       session.Expiration(),
		AnilistAccessToken:  token.AccessToken,
		AnilistRefreshToken: token.RefreshToken,
		AnilistExpiry:       token.Expiry,
	}
	if err = a.db.CreateUser(user); err != nil {
		writeError(w, "error while creating user in db", err)
		return
	}

	if err = a.updateUserMetadata(r.Context(), user); err != nil {
		writeError(w, "error while updating user metadata", err)
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
