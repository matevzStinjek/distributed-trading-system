defmodule ElixirService do
  use GenServer

  # Client API - these functions are called by other processes
  def start_link(_opts) do
    GenServer.start_link(__MODULE__, 0, name: __MODULE__)
  end

  def increment() do
    GenServer.cast(__MODULE__, :increment)
  end

  def get_count() do
    GenServer.call(__MODULE__, :get_count)
  end

  # Server Callbacks - these handle the actual messages
  @impl true
  def init(count) do
    {:ok, count}  # State is just a number
  end

  @impl true
  def handle_cast(:increment, count) do
    {:noreply, count + 1}  # Async, no response needed
  end

  @impl true
  def handle_call(:get_count, _from, count) do
    {:reply, count, count}  # Sync, returns the count
  end

  @impl true
  def handle_info(unexpected, state) do
    IO.puts("Got unexpected message: #{inspect(unexpected)}")
    {:noreply, state}
  end
end
