local counter = {}

function counter.get(store)
  local value, ok = store.get("counter")
  if ok and type(value) == "number" then
    return value
  end
  return 0
end

function counter.set(store, count)
  store.put("counter", count)
end

function counter.increment(store)
  local count = counter.get(store) + 1
  counter.set(store, count)
  return count
end

return counter
