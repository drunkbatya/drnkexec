<template>
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
  </q-page>
</template>

<script setup>
import { ref } from "vue";
import { Notify } from "quasar";
import { useRouter } from "vue-router";
import { login as apiLogin } from "../services/api";

const login = ref("");
const password = ref("");
const loading = ref(false);
const router = useRouter();

const onSubmit = async () => {
  loading.value = true;
  try {
    await apiLogin(login.value, password.value);
    router.push({ name: "main" });
  } catch (e) {
    Notify.create({
      type: "negative",
      message: "Login error",
    });
    console.error(e);
  } finally {
    loading.value = false;
  }
};
</script>
