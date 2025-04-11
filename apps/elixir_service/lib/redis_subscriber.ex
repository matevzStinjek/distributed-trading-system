# lib/your_app/redis_subscriber.ex
defmodule ElixirService.RedisSubscriber do
  use GenServer
  require Logger

  @channel "my-channel-1"

  def start_link(_) do
    GenServer.start_link(__MODULE__, nil, name: __MODULE__)
  end

  @impl true
  def init(_) do
    redis_uri = System.get_env("REDIS_URI") || raise "REDIS_URI environment variable is required"

    # Start subscription connection
    {:ok, pubsub} = Redix.PubSub.start_link(redis_uri, name: :redix_pubsub)

    # Subscribe to channel
    {:ok, _} = Redix.PubSub.subscribe(:redix_pubsub, @channel, self())
    Logger.info("Subscribed to Redis channel: #{@channel}")

    {:ok, %{pubsub: pubsub}}
  end

  @impl true
  def handle_info({:redix_pubsub, _pubsub, _ref, :message, %{channel: channel, payload: payload}}, state) do
    Logger.info("Received message on channel #{channel}: #{payload}")
    {:noreply, state}
  end

  @impl true
  def handle_info({:redix_pubsub, _pubsub, _ref, :subscribed, %{channel: channel}}, state) do
    Logger.info("Successfully subscribed to #{channel}")
    {:noreply, state}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.warning("Unexpected message: #{inspect(msg)}")
    {:noreply, state}
  end
end
