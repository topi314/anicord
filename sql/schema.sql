CREATE TABLE IF NOT EXISTS users
(
    discord_id            BIGINT    NOT NULL PRIMARY KEY,
    discord_access_token  VARCHAR   NOT NULL,
    discord_refresh_token VARCHAR   NOT NULL,
    discord_expiry        TIMESTAMP NOT NULL,
    anilist_access_token  VARCHAR   NOT NULL,
    anilist_refresh_token VARCHAR   NOT NULL,
    anilist_expiry        TIMESTAMP NOT NULL
)
