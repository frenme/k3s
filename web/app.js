const api = (path, options = {}) =>
  fetch(`/api${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  }).then(async (res) => {
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body.error || res.statusText);
    return body;
  });

const statusEl = document.getElementById("status");
const usersEl = document.getElementById("users");
const ordersEl = document.getElementById("orders");
const userSelect = document.querySelector("#order-form select");

