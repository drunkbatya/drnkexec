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

export async function login(login, password) {
  const res = await userApi.post("/login", { login, password }, { skipNotify: true });
  return res.data;
}

export async function logout() {
  await userApi.post("/logout");
}

export async function fetchHosts({ page }) {
  const res = await protectedApi.get("/hosts", {
    params: {
      page,
    },
  });
  return res.data;
}

export async function fetchChecks({ page, hostName, checkName }) {
  const res = await protectedApi.get("/checks", {
    params: {
      page,
      host_name: hostName,
      check_name: checkName,
    },
  });
  return res.data;
}

export async function triggerCheckNow(hostName, checkName) {
  await protectedApi.post("/check/now", {
    host_name: hostName,
    check_name: checkName,
  });
}
