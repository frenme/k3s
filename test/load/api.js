import http from "k6/http";
import { check } from "k6";

const base = __ENV.BASE_URL || "http://gateway:8080";

export const options = {
  scenarios: {
    api: {
      executor: "constant-arrival-rate",
      rate: Number(__ENV.RATE || 200),
      timeUnit: "1s",
      duration: __ENV.DURATION || "1m",
      preAllocatedVUs: 50,
      maxVUs: 400,
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<300"],
  },
};

export default function () {
  const res = http.get(`${base}/api/users`);
  check(res, { "status is 200": (r) => r.status === 200 });
}
