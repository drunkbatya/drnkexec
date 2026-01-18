import axios from "axios";
import { Notify } from "quasar";
import router from "../router";
import { decrementRequests, incrementRequests } from "../services/requestTracker";

const protectedApi = axios.create({
  baseURL: "/api/v1/admin",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true,
});

const userApi = axios.create({
  baseURL: "/api/v1/user",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true,
});

function notifyError(error) {
  const { response } = error || {};
  if (response) {
    const status = response.status;
    const statusText = response.statusText || "Error";
    const detail = response.data?.error || response.data?.message;
    Notify.create({
      type: "negative",
      position: "bottom",
      message: `${status} (${statusText})`,
      caption: detail || error.message || "",
      timeout: 6000,
    });
  } else {
    Notify.create({
      type: "negative",
      position: "bottom",
      message: "Network error",
      caption: error?.message || "Request failed",
      timeout: 6000,
    });
  }
}

const onRequest = (config) => {
  incrementRequests();
  return config;
};

const onRequestError = (error) => {
  decrementRequests();
  notifyError(error);
  return Promise.reject(error);
};

const onResponse = (response) => {
  decrementRequests();
  return response;
};

const onResponseError = (error, redirectOnForbidden = false) => {
  decrementRequests();
  if (redirectOnForbidden && error.response && error.response.status === 403) {
    router.replace({ name: "login" });
  }
  if (!error?.config?.skipNotify) {
    notifyError(error);
  }
  return Promise.reject(error);
};

protectedApi.interceptors.request.use(onRequest, onRequestError);
protectedApi.interceptors.response.use(
  onResponse,
  (error) => onResponseError(error, true)
);

userApi.interceptors.request.use(onRequest, onRequestError);
userApi.interceptors.response.use(onResponse, (error) => onResponseError(error, false));

function encodeStatuses(statuses) {
  if (!Array.isArray(statuses)) {
    return undefined;
  }
  const values = statuses
    .map((status) => (typeof status === "string" ? status.trim() : ""))
    .filter((status) => Boolean(status));
  if (values.length === 0) {
    return undefined;
  }
  return JSON.stringify(values);
}

function normalizeFilterValue(value) {
  if (typeof value !== "string") {
    return "";
  }
  const trimmed = value.trim();
  return trimmed || "";
}

export async function login(login, password) {
  const res = await userApi.post("/login", { login, password }, { skipNotify: true });
  return res.data;
}

export async function logout() {
  await userApi.post("/logout");
}

export async function fetchHosts({ count, offset, hostName, hostNameRegex, statuses } = {}) {
  const params = {};
  if (typeof count === "number") {
    params.count = count;
  }
  if (typeof offset === "number") {
    params.offset = offset;
  }
  const hostExact = normalizeFilterValue(hostName);
  if (hostExact) {
    params.host_name = hostExact;
  }
  const hostPattern = normalizeFilterValue(hostNameRegex);
  if (hostPattern) {
    params.host_name_regex = hostPattern;
  }
  const encodedStatuses = encodeStatuses(statuses);
  if (encodedStatuses) {
    params.statuses = encodedStatuses;
  }
  const res = await protectedApi.get("/hosts", { params });
  return res.data;
}

export async function fetchCheckSummaries({ count, offset, checkName, checkNameRegex, statuses } = {}) {
  const params = {};
  if (typeof count === "number") {
    params.count = count;
  }
  if (typeof offset === "number") {
    params.offset = offset;
  }
  const checkExact = normalizeFilterValue(checkName);
  if (checkExact) {
    params.check_name = checkExact;
  }
  const checkPattern = normalizeFilterValue(checkNameRegex);
  if (checkPattern) {
    params.check_name_regex = checkPattern;
  }
  const encodedStatuses = encodeStatuses(statuses);
  if (encodedStatuses) {
    params.statuses = encodedStatuses;
  }
  const res = await protectedApi.get("/checks", { params });
  return res.data;
}

export async function fetchCheckDetails({ count, offset, hostName, checkName, statuses } = {}) {
  const params = {};
  if (typeof count === "number") {
    params.count = count;
  }
  if (typeof offset === "number") {
    params.offset = offset;
  }
  const hostExact = normalizeFilterValue(hostName);
  if (hostExact) {
    params.host_name = hostExact;
  }
  const checkExact = normalizeFilterValue(checkName);
  if (checkExact) {
    params.check_name = checkExact;
  }
  const encodedStatuses = encodeStatuses(statuses);
  if (encodedStatuses) {
    params.statuses = encodedStatuses;
  }
  const res = await protectedApi.get("/checks/detail", { params });
  return res.data;
}

export async function deleteDowntime(name) {
  await protectedApi.delete("/downtime", {
    data: { name },
  });
}

export async function fetchDowntimes({
  count,
  offset,
  hostName,
  hostNameRegex,
  checkName,
  checkNameRegex,
  name,
  nameRegex,
  scope,
} = {}) {
  const params = {};
  if (typeof count === "number") {
    params.count = count;
  }
  if (typeof offset === "number") {
    params.offset = offset;
  }
  const hostExact = normalizeFilterValue(hostName);
  if (hostExact) {
    params.host_name = hostExact;
  }
  const hostPattern = normalizeFilterValue(hostNameRegex);
  if (hostPattern) {
    params.host_name_regex = hostPattern;
  }
  const checkExact = normalizeFilterValue(checkName);
  if (checkExact) {
    params.check_name = checkExact;
  }
  const checkPattern = normalizeFilterValue(checkNameRegex);
  if (checkPattern) {
    params.check_name_regex = checkPattern;
  }
  const nameExact = normalizeFilterValue(name);
  if (nameExact) {
    params.name = nameExact;
  }
  const namePattern = normalizeFilterValue(nameRegex);
  if (namePattern) {
    params.name_regex = namePattern;
  }
  if (scope && scope !== "all") {
    params.scope = scope;
  }
  const res = await protectedApi.get("/downtime", { params });
  return res.data;
}

export async function createDowntimeRelative({ hostName, checkName, name, duration }) {
  await protectedApi.post("/downtime/relative", {
    host_name: hostName,
    check_name: checkName,
    name,
    duration,
  });
}

export async function createDowntimeAbsolute({ hostName, checkName, name, from, till }) {
  await protectedApi.post("/downtime/absolute", {
    host_name: hostName,
    check_name: checkName,
    name,
    from,
    till,
  });
}

export async function triggerCheckNow(hostName, checkName) {
  await protectedApi.post("/check/now", {
    host_name: hostName,
    check_name: checkName,
  });
}
