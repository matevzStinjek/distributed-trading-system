# lib/your_app/application.ex
defmodule ElixirService.Application do
  use Application
  require Logger

  @impl true
  def start(_type, _args) do
    children = [
      # ElixirService,
      # Start the Redis subscriber
      ElixirService.RedisSubscriber

      # Your other supervisors/workers here
      # {Phoenix.PubSub, name: YourApp.PubSub},
      # YourApp.Endpoint
    ]

    opts = [strategy: :one_for_one, name: ElixirService.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
