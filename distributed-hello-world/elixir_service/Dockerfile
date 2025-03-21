FROM elixir:1.18-alpine

WORKDIR /app

# Install hex package manager
RUN mix local.hex --force && \
    mix local.rebar --force

COPY mix.exs ./
RUN mix deps.get

COPY . .

RUN mix compile

EXPOSE 4000

CMD ["mix", "run", "--no-halt"]