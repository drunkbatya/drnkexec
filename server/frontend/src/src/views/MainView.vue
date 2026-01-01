<template>
  <q-layout view="hH lpR fF">
    <q-header elevated class="bg-primary text-white">
      <q-toolbar>
        <q-toolbar-title class="text-weight-bold">
          DrnkExec
        </q-toolbar-title>
        <q-tabs
          v-model="activeTab"
          dense
          shrink
          stretch
          inline-label
          class="text-white"
        >
          <q-tab name="hosts" label="Hosts" />
          <q-tab name="checks" label="Checks" />
        </q-tabs>
        <q-space />
        <q-btn
          flat
          dense
          icon="menu_book"
          label="API Docs"
          href="/api/docs"
          target="_blank"
        />
        <q-btn
          flat
          dense
          icon="logout"
          label="Logout"
          @click="handleLogout"
          :loading="logoutLoading"
        />
      </q-toolbar>
    </q-header>

    <q-page-container>
      <q-page class="q-pa-md bg-grey-1">
        <q-tab-panels v-model="activeTab" animated>
          <q-tab-panel name="hosts" class="q-pa-none">
            <q-card flat bordered>
              <q-card-section class="row items-center">
                <div class="text-h6">Hosts</div>
                <q-space />
                <q-btn
                  flat
                  dense
                  icon="refresh"
                  @click="loadChecks"
                  :loading="loadingChecks"
                />
              </q-card-section>
              <q-separator />
              <div class="q-pa-md">
                <div v-if="loadingChecks" class="text-center q-my-lg">
                  <q-spinner-dots color="primary" size="2rem" />
                </div>
                <div v-else-if="pagedHostGroups.length === 0" class="text-grey-7 text-center">
                  No hosts to display
                </div>
                <q-list v-else bordered class="rounded-borders bg-white">
                  <q-expansion-item
                    v-for="group in pagedHostGroups"
                    :key="group.hostname"
                    v-model="hostExpanded[group.hostname]"
                    expand-separator
                    header-class="bg-grey-2 text-dark text-weight-medium"
                  >
                    <template #header>
                      <q-item-section avatar>
                        <q-icon name="dns" />
                      </q-item-section>
                      <q-item-section>
                        <div class="text-subtitle1">{{ group.hostname }}</div>
                        <div class="text-caption text-grey-7">
                          {{ group.items.length }} checks
                        </div>
                      </q-item-section>
                    </template>

                    <q-table
                      flat
                      dense
                      :rows="group.items"
                      :columns="hostCheckColumns"
                      row-key="CheckName"
                      hide-bottom
                    >
                      <template #body-cell-status="props">
                        <q-td :props="props">
                          <q-badge
                            :color="statusColor(props.row.Status)"
                            :label="props.row.Status?.toUpperCase() || 'UNKNOWN'"
                            align="middle"
                          />
                        </q-td>
                      </template>
                      <template #body-cell-updatedAt="props">
                        <q-td :props="props">
                          {{ formatDate(props.row.UpdatedAt) }}
                        </q-td>
                      </template>
                      <template #body-cell-fail="props">
                        <q-td :props="props">
                          {{ props.row.FailCount || 0 }} / {{ props.row.FailThreshold || 0 }}
                        </q-td>
                      </template>
                      <template #body-cell-output="props">
                        <q-td :props="props">
                          <div class="text-body2">{{ props.row.Output || "—" }}</div>
                        </q-td>
                      </template>
                      <template #body-cell-actions="props">
                        <q-td :props="props">
                          <q-btn
                            size="sm"
                            flat
                            color="primary"
                            icon="play_arrow"
                            label="Check Now"
                            :loading="isCheckRunning(props.row)"
                            @click="runCheckNow(props.row)"
                          />
                        </q-td>
                      </template>
                    </q-table>
                  </q-expansion-item>
                </q-list>
              </div>
              <q-separator />
              <div class="row justify-between items-center q-pa-sm">
                <div class="text-caption text-grey-7">
                  Showing {{ hostPagination.count }} of {{ hostPagination.rowsNumber }} hosts
                </div>
                <q-pagination
                  v-model="hostPagination.page"
                  :max="hostMaxPage"
                  color="primary"
                  boundary-numbers
                  :max-pages="6"
                  dense
                  @update:model-value="changeHostPage"
                />
              </div>
            </q-card>
          </q-tab-panel>

          <q-tab-panel name="checks" class="q-pa-none">
            <q-card flat bordered>
              <q-card-section class="row items-center">
                <div class="text-h6">Checks</div>
                <q-space />
                <q-btn
                  flat
                  dense
                  icon="refresh"
                  @click="loadChecks"
                  :loading="loadingChecks"
                />
              </q-card-section>
              <q-separator />
              <div class="q-pa-md">
                <div v-if="loadingChecks" class="text-center q-my-lg">
                  <q-spinner-dots color="primary" size="2rem" />
                </div>
                <div v-else-if="checkGroups.length === 0" class="text-grey-7 text-center">
                  No checks for current page
                </div>
                <q-list v-else bordered class="rounded-borders bg-white">
                  <q-expansion-item
                    v-for="group in checkGroups"
                    :key="group.checkName"
                    v-model="checkExpanded[group.checkName]"
                    expand-separator
                    header-class="bg-grey-2 text-dark text-weight-medium"
                  >
                    <template #header>
                      <q-item-section avatar>
                        <q-icon name="dns" />
                      </q-item-section>
                      <q-item-section>
                        <div class="text-subtitle1">{{ group.checkName }}</div>
                        <div class="text-caption text-grey-7">
                          {{ group.items.length }} hosts
                        </div>
                      </q-item-section>
                    </template>

                    <q-table
                      flat
                      dense
                      :rows="group.items"
                      :columns="checkHostColumns"
                      row-key="Hostname"
                      hide-bottom
                    >
                      <template #body-cell-status="props">
                        <q-td :props="props">
                          <q-badge
                            :color="statusColor(props.row.Status)"
                            :label="props.row.Status?.toUpperCase() || 'UNKNOWN'"
                            align="middle"
                          />
                        </q-td>
                      </template>
                      <template #body-cell-updatedAt="props">
                        <q-td :props="props">
                          {{ formatDate(props.row.UpdatedAt) }}
                        </q-td>
                      </template>
                      <template #body-cell-fail="props">
                        <q-td :props="props">
                          {{ props.row.FailCount || 0 }} / {{ props.row.FailThreshold || 0 }}
                        </q-td>
                      </template>
                      <template #body-cell-output="props">
                        <q-td :props="props">
                          <div class="text-body2">{{ props.row.Output || "—" }}</div>
                        </q-td>
                      </template>
                      <template #body-cell-actions="props">
                        <q-td :props="props">
                          <q-btn
                            size="sm"
                            flat
                            color="primary"
                            icon="play_arrow"
                            label="Check Now"
                            :loading="isCheckRunning(props.row)"
                            @click="runCheckNow(props.row)"
                          />
                        </q-td>
                      </template>
                    </q-table>
                  </q-expansion-item>
                </q-list>
              </div>
              <q-separator />
              <div class="row justify-between items-center q-pa-sm">
                <div class="text-caption text-grey-7">
                  Showing {{ checkPagination.count }} of {{ checkPagination.rowsNumber }} checks
                </div>
                <q-pagination
                  v-model="checkPagination.page"
                  :max="checkMaxPage"
                  color="primary"
                  boundary-numbers
                  :max-pages="6"
                  dense
                  @update:model-value="loadChecks"
                />
              </div>
            </q-card>
          </q-tab-panel>
        </q-tab-panels>
        <q-inner-loading :showing="globalLoading" color="primary" size="64px" />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Notify } from "quasar";
import { fetchChecks, logout, triggerCheckNow } from "../services/api";
import { TABLE_BATCH_SIZE } from "../config";
import { activeRequests } from "../services/requestTracker";

const router = useRouter();
const globalLoading = computed(() => activeRequests.value > 0);

const activeTab = ref("hosts");
const logoutLoading = ref(false);

const hostPagination = reactive({
  page: 1,
  rowsPerPage: TABLE_BATCH_SIZE,
  rowsNumber: 0,
  count: 0,
});

const checkPagination = reactive({
  page: 1,
  rowsPerPage: TABLE_BATCH_SIZE,
  rowsNumber: 0,
  count: 0,
});
const checkActions = reactive({});
const hostExpanded = reactive({});
const checkExpanded = reactive({});

const checks = ref([]);
const loadingChecks = ref(false);

const hostCheckColumns = [
  {
    name: "check",
    label: "Check",
    field: (row) => row.CheckName,
    align: "left",
  },
  {
    name: "status",
    label: "Status",
    field: (row) => row.Status,
    align: "left",
  },
  {
    name: "updatedAt",
    label: "Last Check",
    field: (row) => row.UpdatedAt,
    align: "left",
  },
  {
    name: "fail",
    label: "Fails",
    field: (row) => row.FailCount,
    align: "right",
  },
  {
    name: "output",
    label: "Output",
    field: (row) => row.Output,
    align: "left",
  },
  {
    name: "actions",
    label: "Actions",
    align: "right",
  },
];

const checkHostColumns = [
  {
    name: "hostname",
    label: "Host",
    field: (row) => row.Hostname,
    align: "left",
  },
  {
    name: "status",
    label: "Status",
    field: (row) => row.Status,
    align: "left",
  },
  {
    name: "updatedAt",
    label: "Last Check",
    field: (row) => row.UpdatedAt,
    align: "left",
  },
  {
    name: "fail",
    label: "Fails",
    field: (row) => row.FailCount,
    align: "right",
  },
  {
    name: "output",
    label: "Output",
    field: (row) => row.Output,
    align: "left",
  },
  {
    name: "actions",
    label: "Actions",
    align: "right",
  },
];

const hostMaxPage = computed(() => {
  const total = hostPagination.rowsNumber || 0;
  const perPage = hostPagination.rowsPerPage || TABLE_BATCH_SIZE;
  if (total === 0) {
    return 1;
  }
  return Math.ceil(total / perPage);
});

const checkMaxPage = computed(() => {
  const total = checkPagination.rowsNumber || 0;
  const perPage = checkPagination.rowsPerPage || TABLE_BATCH_SIZE;
  if (total === 0) {
    return 1;
  }
  return Math.ceil(total / perPage);
});

const hostGroups = computed(() => {
  const groups = [];
  const index = new Map();
  checks.value.forEach((check) => {
    const hostname = check.Hostname || "unknown";
    if (!index.has(hostname)) {
      const entry = { hostname, items: [] };
      index.set(hostname, entry);
      groups.push(entry);
    }
    index.get(hostname).items.push(check);
  });
  return groups;
});

const pagedHostGroups = computed(() => {
  const groups = hostGroups.value;
  const perPage = hostPagination.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, hostPagination.page);
  const start = (page - 1) * perPage;
  const sliced = groups.slice(start, start + perPage);
  hostPagination.count = sliced.length;
  return sliced;
});

const checkGroups = computed(() => {
  const groups = [];
  const index = new Map();
  checks.value.forEach((check) => {
    const checkName = check.CheckName || "unknown";
    if (!index.has(checkName)) {
      const entry = { checkName, items: [] };
      index.set(checkName, entry);
      groups.push(entry);
    }
    index.get(checkName).items.push(check);
  });
  return groups;
});

onMounted(() => {
  loadChecks();
});

function changeHostPage(nextPage) {
  hostPagination.page = nextPage;
}

async function loadChecks(nextPage) {
  if (typeof nextPage === "number") {
    checkPagination.page = nextPage;
  }
  loadingChecks.value = true;
  try {
    const data = await fetchChecks({
      page: checkPagination.page,
    });
    checks.value = data.items || [];
    checkPagination.count = data.count ?? checks.value.length;
    checkPagination.rowsNumber = data.total ?? checkPagination.count;
    if (data.page && data.page !== checkPagination.page) {
      checkPagination.page = data.page;
    }
  } catch (err) {
    console.error("load checks", err);
  } finally {
    loadingChecks.value = false;
  }
}

watch(
  hostGroups,
  (groups) => {
    hostPagination.rowsNumber = groups.length;
    const max = hostMaxPage.value;
    if (hostPagination.page > max) {
      hostPagination.page = max;
    }
    if (hostPagination.page <= 0) {
      hostPagination.page = 1;
    }
    const allowed = new Set(groups.map((group) => group.hostname));
    Object.keys(hostExpanded).forEach((key) => {
      if (!allowed.has(key)) {
        delete hostExpanded[key];
      }
    });
  },
  { immediate: true }
);

watch(
  checkGroups,
  (groups) => {
    const allowed = new Set(groups.map((group) => group.checkName));
    Object.keys(checkExpanded).forEach((key) => {
      if (!allowed.has(key)) {
        delete checkExpanded[key];
      }
    });
  },
  { immediate: true }
);

function statusColor(status) {
  switch (status) {
    case "ok":
      return "positive";
    case "warning":
      return "warning";
    case "critical":
      return "negative";
    default:
      return "grey";
  }
}

function formatDate(value) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

async function handleLogout() {
  logoutLoading.value = true;
  try {
    await logout();
    router.replace({ name: "login" });
  } catch (err) {
    console.error("logout failed", err);
  } finally {
    logoutLoading.value = false;
  }
}

function actionKey(row) {
  return `${row.Hostname || ""}::${row.CheckName || ""}`;
}

function isCheckRunning(row) {
  return !!checkActions[actionKey(row)];
}

async function runCheckNow(row) {
  const key = actionKey(row);
  if (checkActions[key]) {
    return;
  }
  checkActions[key] = true;
  try {
    await triggerCheckNow(row.Hostname, row.CheckName);
    Notify.create({
      type: "positive",
      message: `Triggered ${row.CheckName} on ${row.Hostname}`,
    });
    await loadChecks();
  } catch (err) {
    console.error("check now failed", err);
    Notify.create({
      type: "negative",
      message: "Failed to trigger check",
    });
  } finally {
    delete checkActions[key];
  }
}
</script>
