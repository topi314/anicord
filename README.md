# Anicord
Anicord adds an anilist connection to linked roles and updates the metadata every 12h. Keep in mind that https://anilist.co only updates your stats every 48h.

## Setup

Invite the bot [here](https://discord.com/api/oauth2/authorize?client_id=1062741787992658070&scope=bot%20applications.commands)

Go to `Server` -> `Settings` -> `Roles` -> `Links` -> `Add requirement` -> `Anicord`

Input the minimal amount of watched anime or read manga to be eligible for the role.

Go to `Server` -> `Linked Roles` -> Click on the role & login with discord & then anilist.

Now Anicord will try to update the metadata every 12h.

## Installation

### Requirements
    1. PostgreSQL Database/SQLite
    2. Docker/Linux
    3. Go 1.21+

### Building

get the docker image `ghcr.io/topi314/anicord` or install it via `go install github.com/topi314/anicord@latest`

### Configuration

See `config.example.yml` for all available configuration options.

### Running

Execute the `anicord` binary or run the docker image

# Help
If you encounter any problems feel free to open an issue or reach out to me(`topi314`) via discord [here](https://discord.gg/RKM92xXu4Y)
