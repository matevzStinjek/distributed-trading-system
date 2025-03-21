defmodule ElixirServiceTest do
  use ExUnit.Case
  doctest ElixirService

  test "greets the world" do
    assert ElixirService.hello() == :world
  end
end
