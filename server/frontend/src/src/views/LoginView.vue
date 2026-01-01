<template>
  <q-layout view="hH lpR fF">
    <q-page-container>
      <q-page class="flex flex-center bg-grey-2">
        <q-card class="q-pa-lg" style="width: 400px; max-width: 90vw">
          <div class="text-h5 text-center q-mb-md">DrnkExec Login</div>

          <q-form @submit.prevent="onSubmit">
            <q-input
              v-model="login"
              label="Login"
              outlined
              dense
              class="q-mb-md"
              :disable="loading"
            >
              <template #prepend>
                <q-icon name="person" />
              </template>
            </q-input>

            <q-input
              v-model="password"
              label="Password"
              type="password"
              outlined
              dense
              class="q-mb-md"
              :disable="loading"
            >
              <template #prepend>
                <q-icon name="lock" />
              </template>
            </q-input>

            <q-btn
              type="submit"
              label="Login"
              color="primary"
              unelevated
              class="full-width q-mt-sm"
              :loading="loading"
            />
          </q-form>
        </q-card>
        <q-inner-loading :showing="globalLoading" color="primary" size="64px" />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { computed, ref } from "vue";
import { Notify } from "quasar";
import { useRouter } from "vue-router";
import { login as apiLogin } from "../services/api";
import { activeRequests } from "../services/requestTracker";

const login = ref("");
const password = ref("");
const loading = ref(false);
const router = useRouter();
const globalLoading = computed(() => activeRequests.value > 0);

const onSubmit = async () => {
  loading.value = true;
  try {
    await apiLogin(login.value, password.value);
    router.push({ name: "main" });
  } catch (e) {
    const status = e?.response?.status;
    const statusText = e?.response?.statusText || "Error";
    const detail = e?.response?.data?.error || e?.response?.data?.message || e?.message;
    if (status === 401 || status === 403) {
      Notify.create({
        type: "negative",
        position: "bottom",
        message: "Login error",
        caption: detail || "Invalid credentials",
        timeout: 6000,
      });
    } else {
      Notify.create({
        type: "negative",
        position: "bottom",
        message: `${status || ""} ${statusText}`.trim(),
        caption: detail || "Request failed",
        timeout: 6000,
      });
    }
    console.error("login error", e);
  } finally {
    loading.value = false;
  }
};
</script>
