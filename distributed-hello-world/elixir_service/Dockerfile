FROM elixir:1.18-alpine

WORKDIR /app

RUN mix local.hex --force && \
    mix local.rebar --force

CMD ["mix", "run", "--no-halt"]
