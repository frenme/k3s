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

const setStatus = (text) => {
  statusEl.textContent = text;
};

const li = (html) => {
  const el = document.createElement("li");
  el.innerHTML = html;
  return el;
};

const esc = (s) =>
  String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

async function refresh() {
  const [users, orders] = await Promise.all([api("/users/users"), api("/orders/orders")]);
  usersEl.replaceChildren(...users.items.map((u) => li(`${esc(u.name)} <small>${esc(u.email)}</small>`)));
  userSelect.replaceChildren(
    ...users.items.map((u) => new Option(`${u.name} (${u.email})`, u.id)),
  );
  const names = Object.fromEntries(users.items.map((u) => [u.id, u.name]));
  ordersEl.replaceChildren(
    ...orders.items.map((o) =>
      li(`${esc(o.item)} <small>${(o.amount_cents / 100).toFixed(2)} by ${esc(names[o.user_id] || o.user_id)}</small>`),
    ),
  );
}

