package anicord

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/oauth2"
	"github.com/disgoorg/snowflake/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func NewDB(ctx context.Context, cfg DBConfig, schema string) (*DB, error) {
	var (
		driverName     string
		dataSourceName string
	)
	switch cfg.Type {
	case "postgresql":
		driverName = "pgx"
		pgCfg, err := pgx.ParseConfig(cfg.PostgresDataSourceName())
		if err != nil {
			return nil, fmt.Errorf("failed to parse postgres config: %w", err)
		}

		dataSourceName = stdlib.RegisterConnConfig(pgCfg)

	case "sqlite":
		driverName = "sqlite"
		dataSourceName = cfg.Path

	default:
		return nil, errors.New("invalid database type, must be one of: postgres, sqlite")
	}

	dbx, err := sqlx.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = dbx.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	// execute schema
	if _, err = dbx.ExecContext(ctx, schema); err != nil {
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	return &DB{dbx: dbx}, nil
}

type DB struct {
	dbx *sqlx.DB
}

func (d *DB) Close() error {
	return d.dbx.Close()
}

func (d *DB) GetUser(ctx context.Context, userID snowflake.ID) (*User, error) {
	var user User
	err := d.dbx.GetContext(ctx, &user, "SELECT * FROM users WHERE discord_id = $1", userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *DB) GetUsers(ctx context.Context) ([]User, error) {
	var users []User
	err := d.dbx.SelectContext(ctx, &users, "SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (d *DB) AddUser(ctx context.Context, user User) error {
	_, err := d.dbx.NamedExecContext(ctx, `
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

func (d *DB) RemoveUser(ctx context.Context, userID snowflake.ID) error {
	_, err := d.dbx.ExecContext(ctx, "DELETE FROM users WHERE discord_id = $1", userID)
	return err
}

type User struct {
	DiscordID           snowflake.ID `db:"discord_id"`
	DiscordAccessToken  string       `db:"discord_access_token"`
	DiscordRefreshToken string       `db:"discord_refresh_token"`
	DiscordExpiry       time.Time    `db:"discord_expiry"`

	AnilistAccessToken  string    `db:"anilist_access_token"`
	AnilistRefreshToken string    `db:"anilist_refresh_token"`
	AnilistExpiry       time.Time `db:"anilist_expiry"`
}

func (u User) Session() oauth2.Session {
	return oauth2.Session{
		AccessToken:  u.DiscordAccessToken,
		RefreshToken: u.DiscordRefreshToken,
		Scopes:       discordOAuth2Scopes,
		TokenType:    discord.TokenTypeBearer,
		Expiration:   u.DiscordExpiry,
	}
}
