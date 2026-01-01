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
                  @click="refreshHosts"
                  :loading="loadingHosts"
                />
              </q-card-section>
              <q-separator />
              <div class="q-pa-md">
                <div v-if="loadingHosts" class="text-center q-my-lg">
                  <q-spinner-dots color="primary" size="2rem" />
                </div>
                <div v-else-if="hosts.length === 0" class="text-grey-7 text-center">
                  No hosts to display
                </div>
                <q-list v-else bordered class="rounded-borders bg-white">
                  <q-expansion-item
                    v-for="host in hosts"
                    :key="host.Hostname"
                    v-model="hostExpanded[host.Hostname]"
                    @show="() => loadHostChecks(host.Hostname)"
                    expand-separator
                    header-class="bg-grey-2 text-dark text-weight-medium"
                  >
                    <template #header>
                      <q-item-section avatar>
                        <q-icon name="dns" :color="hostStatusColor(host)" />
                      </q-item-section>
                      <q-item-section>
                        <div class="text-subtitle1">{{ host.Hostname }}</div>
                        <div class="text-caption text-grey-7">
                          {{ host.CheckCount || 0 }} checks —
                          OK: {{ host.OK || 0 }},
                          Warning: {{ host.Warning || 0 }},
                          Critical: {{ host.Critical || 0 }},
                          Unknown: {{ host.Unknown || 0 }}
                        </div>
                      </q-item-section>
                    </template>

                    <div v-if="getHostChecksState(host.Hostname).loading" class="text-center q-my-lg">
                      <q-spinner-dots color="primary" size="2rem" />
                    </div>
                    <div
                      v-else-if="getHostChecksState(host.Hostname).items.length === 0"
                      class="text-grey-7 text-center q-my-md"
                    >
                      No checks to display
                    </div>
                    <template v-else>
                      <q-table
                        flat
                        dense
                        :rows="getHostChecksState(host.Hostname).items"
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
                      <div class="row justify-between items-center q-pa-sm">
                        <div class="text-caption text-grey-7">
                          Showing {{ getHostChecksState(host.Hostname).count }}
                          of {{ getHostChecksState(host.Hostname).rowsNumber }} checks
                        </div>
                        <q-pagination
                          :model-value="getHostChecksState(host.Hostname).page"
                          :max="hostChecksMaxPage(host.Hostname)"
                          color="primary"
                          boundary-numbers
                          :max-pages="6"
                          dense
                          @update:model-value="(page) => changeHostChecksPage(host.Hostname, page)"
                        />
                      </div>
                    </template>
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
import { fetchChecks, fetchHosts, logout, triggerCheckNow } from "../services/api";
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

const hosts = ref([]);
const loadingHosts = ref(false);
const hostChecksState = reactive({});
const defaultHostChecksState = {
  page: 1,
  rowsPerPage: TABLE_BATCH_SIZE,
  rowsNumber: 0,
  count: 0,
  items: [],
  loading: false,
};

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
  loadHosts();
  loadChecks();
});

async function loadHosts(nextPage, options = {}) {
  const refreshOpen = options.refreshOpen === true;
  if (typeof nextPage === "number") {
    hostPagination.page = nextPage;
  }
  const perPage = hostPagination.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, hostPagination.page);
  const offset = (page - 1) * perPage;
  loadingHosts.value = true;
  try {
    const data = await fetchHosts({ count: perPage, offset });
    hosts.value = data.hosts || [];
    hostPagination.count = data.count ?? hosts.value.length;
    hostPagination.rowsNumber = data.total ?? hostPagination.count;
    const maxPage = hostMaxPage.value;
    if (page > maxPage && maxPage > 0) {
      hostPagination.page = maxPage;
      if (maxPage !== page) {
        await loadHosts(maxPage, options);
        return;
      }
    }
    if (refreshOpen) {
      const expandedHosts = hosts.value
        .filter((host) => hostExpanded[host.Hostname])
        .map((host) => host.Hostname);
      await Promise.all(expandedHosts.map((hostname) => loadHostChecks(hostname, getHostChecksState(hostname).page)));
    }
  } catch (err) {
    console.error("load hosts", err);
  } finally {
    loadingHosts.value = false;
  }
}

function refreshHosts() {
  loadHosts(undefined, { refreshOpen: true });
}

function changeHostPage(nextPage) {
  loadHosts(nextPage);
}

async function loadChecks(nextPage) {
  if (typeof nextPage === "number") {
    checkPagination.page = nextPage;
  }
  const perPage = checkPagination.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, checkPagination.page);
  const offset = (page - 1) * perPage;
  loadingChecks.value = true;
  try {
    const data = await fetchChecks({
      count: perPage,
      offset,
    });
    checks.value = data.items || [];
    checkPagination.count = data.count ?? checks.value.length;
    checkPagination.rowsNumber = data.total ?? checkPagination.count;
    const maxPage = checkMaxPage.value;
    if (page > maxPage && maxPage > 0) {
      checkPagination.page = maxPage;
      if (maxPage !== page) {
        await loadChecks(maxPage);
      }
    }
  } catch (err) {
    console.error("load checks", err);
  } finally {
    loadingChecks.value = false;
  }
}

function createHostChecksState() {
  return reactive({
    page: 1,
    rowsPerPage: TABLE_BATCH_SIZE,
    rowsNumber: 0,
    count: 0,
    items: [],
    loading: false,
  });
}

function resolveHostChecksState(hostname) {
  if (!hostname) {
    return createHostChecksState();
  }
  if (!hostChecksState[hostname]) {
    hostChecksState[hostname] = createHostChecksState();
  }
  return hostChecksState[hostname];
}

function getHostChecksState(hostname) {
  if (!hostname) {
    return defaultHostChecksState;
  }
  return hostChecksState[hostname] || defaultHostChecksState;
}

async function loadHostChecks(hostname, nextPage) {
  if (!hostname) {
    return;
  }
  const state = resolveHostChecksState(hostname);
  if (typeof nextPage === "number") {
    state.page = nextPage;
  }
  const perPage = state.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, state.page);
  const offset = (page - 1) * perPage;
  state.loading = true;
  try {
    const data = await fetchChecks({
      hostName: hostname,
      count: perPage,
      offset,
    });
    state.items = data.items || [];
    state.count = data.count ?? state.items.length;
    state.rowsNumber = data.total ?? state.count;
    const maxPage = hostChecksMaxPage(hostname);
    if (page > maxPage && maxPage > 0) {
      state.page = maxPage;
      if (maxPage !== page) {
        await loadHostChecks(hostname, maxPage);
      }
    }
  } catch (err) {
    console.error("load host checks", err);
  } finally {
    state.loading = false;
  }
}

function hostChecksMaxPage(hostname) {
  const state = getHostChecksState(hostname);
  const total = state.rowsNumber || 0;
  const perPage = state.rowsPerPage || TABLE_BATCH_SIZE;
  if (total === 0) {
    return 1;
  }
  return Math.ceil(total / perPage);
}

function changeHostChecksPage(hostname, nextPage) {
  loadHostChecks(hostname, nextPage);
}

watch(
  hosts,
  (list) => {
    const allowed = new Set(list.map((host) => host.Hostname));
    Object.keys(hostExpanded).forEach((key) => {
      if (!allowed.has(key)) {
        delete hostExpanded[key];
      }
    });
    Object.keys(hostChecksState).forEach((key) => {
      if (!allowed.has(key)) {
        delete hostChecksState[key];
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

function hostStatusColor(host) {
  if (!host) {
    return "grey";
  }
  if (host.Critical > 0) {
    return "negative";
  }
  if (host.Warning > 0) {
    return "warning";
  }
  if (host.OK > 0 && (host.Unknown === 0 || !host.Unknown)) {
    return "positive";
  }
  if (host.Unknown > 0) {
    return "grey";
  }
  return "grey";
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
    await Promise.all([loadHosts(hostPagination.page), loadChecks(checkPagination.page)]);
    const hostVisible = hosts.value.some((host) => host.Hostname === row.Hostname);
    if (hostVisible) {
      await loadHostChecks(row.Hostname, getHostChecksState(row.Hostname).page);
    }
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
