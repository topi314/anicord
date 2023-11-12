package anicord

import (
	"math/rand"
	"net/http"

	"github.com/disgoorg/disgo/discord"
)

var (
	discordOAuth2Scopes = []discord.OAuth2Scope{discord.OAuth2ScopeIdentify, discord.OAuth2ScopeRoleConnectionsWrite}
	letters             = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func (b *Bot) handleVerify(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, b.oauth2.GenerateAuthorizationURL(b.cfg.Server.BaseURL+"/discord", discord.PermissionsNone, 0, false, discord.OAuth2ScopeIdentify, discord.OAuth2ScopeRoleConnectionsWrite), http.StatusTemporaryRedirect)
}

func (b *Bot) handleDiscord(w http.ResponseWriter, r *http.Request) {
	var (
		query = r.URL.Query()
		code  = query.Get("code")
		state = query.Get("state")
	)
	if code == "" || state == "" {
		writeError(w, "invalid request", nil)
		return
	}

	anilistState := randStr(32)
	session, _, err := b.oauth2.StartSession(code, state)
	if err != nil {
		writeError(w, "error while starting session", err)
		return
	}
	b.sessions[anilistState] = session

	http.Redirect(w, r, b.anilist.AuthCodeURL(anilistState), http.StatusTemporaryRedirect)
}

func (b *Bot) handleAnilist(w http.ResponseWriter, r *http.Request) {
	var (
		query = r.URL.Query()
		code  = query.Get("code")
		state = query.Get("state")
	)
	if code == "" || state == "" {
		writeError(w, "invalid request", nil)
		return
	}

	session, ok := b.sessions[state]
	if !ok {
		writeError(w, "invalid session", nil)
		return
	}

	discordUser, err := b.oauth2.GetUser(session)
	if err != nil {
		writeError(w, "error while getting user", err)
		return
	}

	token, err := b.anilist.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, "error while exchanging token", err)
		return
	}

	user := User{
		DiscordID:           discordUser.ID,
		DiscordAccessToken:  session.AccessToken,
		DiscordRefreshToken: session.RefreshToken,
		DiscordExpiry:       session.Expiration,
		AnilistAccessToken:  token.AccessToken,
		AnilistRefreshToken: token.RefreshToken,
		AnilistExpiry:       token.Expiry,
	}
	if err = b.db.AddUser(r.Context(), user); err != nil {
		writeError(w, "error while creating user in db", err)
		return
	}

	if err = b.updateUserMetadata(r.Context(), user); err != nil {
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
