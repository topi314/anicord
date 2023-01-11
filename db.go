package main

import (
	"fmt"
	"os"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/snowflake/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	dbHost     = os.Getenv("DB_HOST")
	dbPort     = os.Getenv("DB_PORT")
	dbUser     = os.Getenv("DB_USER")
	dbPassword = os.Getenv("DB_PASSWORD")
	dbName     = os.Getenv("DB_NAME")
	dbSSLMode  = os.Getenv("DB_SSLMODE")
)

func NewDB() (*DB, error) {
	db, err := sqlx.Connect("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode))
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

type DB struct {
	db *sqlx.DB
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetUser(userID snowflake.ID) (*User, error) {
	var user User
	err := d.db.Get(&user, `SELECT * FROM users WHERE discord_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *DB) GetUsers() ([]User, error) {
	var users []User
	err := d.db.Select(&users, `SELECT * FROM users`)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (d *DB) CreateUser(user User) error {
	_, err := d.db.NamedExec(`
		INSERT INTO users (
			discord_id,
			discord_access_token,
			discord_refresh_token,
			discord_expiry,
			anilist_access_token,
			anilist_refresh_token,
			anilist_expiry
		) VALUES (
			:discord_id,
			:discord_access_token,
			:discord_refresh_token,
			:discord_expiry,
			:anilist_access_token,
			:anilist_refresh_token,
			:anilist_expiry
	  	) ON CONFLICT (discord_id) DO UPDATE SET 
			discord_access_token = :discord_access_token,
			discord_refresh_token = :discord_refresh_token,
			discord_expiry = :discord_expiry,
			anilist_access_token = :anilist_access_token,
			anilist_refresh_token = :anilist_refresh_token,
			anilist_expiry = :anilist_expiry 
		`, user)
	return err
}

func (d *DB) DeleteUser(userID snowflake.ID) error {
	_, err := d.db.Exec(`DELETE FROM users WHERE discord_id = $1`, userID)
	return err
}

var _ oauth2.Session = (*User)(nil)

type User struct {
	DiscordID           snowflake.ID `db:"discord_id"`
	DiscordAccessToken  string       `db:"discord_access_token"`
	DiscordRefreshToken string       `db:"discord_refresh_token"`
	DiscordExpiry       time.Time    `db:"discord_expiry"`

	AnilistAccessToken  string    `db:"anilist_access_token"`
	AnilistRefreshToken string    `db:"anilist_refresh_token"`
	AnilistExpiry       time.Time `db:"anilist_expiry"`
}

func (u User) AccessToken() string {
	return u.DiscordAccessToken
}

func (u User) RefreshToken() string {
	return u.DiscordRefreshToken
}

func (u User) Scopes() []discord.OAuth2Scope {
	return []discord.OAuth2Scope{discord.OAuth2ScopeIdentify, discord.OAuth2ScopeRoleConnectionsWrite}
}

func (u User) TokenType() discord.TokenType {
	return discord.TokenTypeBearer
}

func (u User) Expiration() time.Time {
	return u.DiscordExpiry
}

func (u User) Webhook() *discord.IncomingWebhook {
	return nil
}
