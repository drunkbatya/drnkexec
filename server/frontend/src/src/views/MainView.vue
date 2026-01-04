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
                <q-input
                  v-model="hostSearch"
                  label="Filter by host name"
                  dense
                  outlined
                  clearable
                  debounce="0"
                  @update:model-value="handleHostSearchInput"
                  class="q-mb-md"
                >
                  <template #prepend>
                    <q-icon name="search" />
                  </template>
                </q-input>
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
                            <button
                              class="date-toggle"
                              type="button"
                              @click="toggleHostDateMode(hostKey(props.row))"
                            >
                              {{ formattedDate(props.row.UpdatedAt, hostDateModes[hostKey(props.row)]) }}
                            </button>
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
                  @click="refreshChecks"
                  :loading="loadingChecks"
                />
              </q-card-section>
              <q-separator />
              <div class="q-pa-md">
                <q-input
                  v-model="checkSearch"
                  label="Filter by check name"
                  dense
                  outlined
                  clearable
                  debounce="0"
                  @update:model-value="handleCheckSearchInput"
                  class="q-mb-md"
                >
                  <template #prepend>
                    <q-icon name="search" />
                  </template>
                </q-input>
                <div v-if="loadingChecks" class="text-center q-my-lg">
                  <q-spinner-dots color="primary" size="2rem" />
                </div>
                <div v-else-if="checkGroups.length === 0" class="text-grey-7 text-center">
                  No checks for current page
                </div>
                <q-list v-else bordered class="rounded-borders bg-white">
                  <q-expansion-item
                    v-for="group in checkGroups"
                    :key="group.CheckName"
                    v-model="checkExpanded[group.CheckName]"
                    @show="() => loadCheckDetails(group.CheckName)"
                    expand-separator
                    header-class="bg-grey-2 text-dark text-weight-medium"
                  >
                    <template #header>
                      <q-item-section avatar>
                        <q-icon name="fact_check" :color="hostStatusColor(group)" />
                      </q-item-section>
                      <q-item-section>
                        <div class="text-subtitle1">{{ group.CheckName }}</div>
                        <div class="text-caption text-grey-7">
                          Hosts: {{ group.HostCount || 0 }} —
                          OK: {{ group.OK || 0 }},
                          Warning: {{ group.Warning || 0 }},
                          Critical: {{ group.Critical || 0 }},
                          Unknown: {{ group.Unknown || 0 }}
                        </div>
                      </q-item-section>
                    </template>

                    <div v-if="getCheckDetailsState(group.CheckName).loading" class="text-center q-my-lg">
                      <q-spinner-dots color="primary" size="2rem" />
                    </div>
                    <div
                      v-else-if="getCheckDetailsState(group.CheckName).items.length === 0"
                      class="text-grey-7 text-center q-my-md"
                    >
                      No hosts to display
                    </div>
                    <template v-else>
                      <q-table
                        flat
                        dense
                        :rows="getCheckDetailsState(group.CheckName).items"
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
                            <button
                              class="date-toggle"
                              type="button"
                              @click="toggleCheckDateMode(checkKey(props.row))"
                            >
                              {{ formattedDate(props.row.UpdatedAt, checkDateModes[checkKey(props.row)]) }}
                            </button>
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
                          Showing {{ getCheckDetailsState(group.CheckName).count }}
                          of {{ getCheckDetailsState(group.CheckName).rowsNumber }} hosts
                        </div>
                        <q-pagination
                          :model-value="getCheckDetailsState(group.CheckName).page"
                          :max="checkDetailsMaxPage(group.CheckName)"
                          color="primary"
                          boundary-numbers
                          :max-pages="6"
                          dense
                          @update:model-value="(page) => changeCheckDetailsPage(group.CheckName, page)"
                        />
                      </div>
                    </template>
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
                  @update:model-value="loadCheckSummaries"
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
import { fetchCheckDetails, fetchCheckSummaries, fetchHosts, logout, triggerCheckNow } from "../services/api";
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
const hostSearch = ref("");
let hostSearchTimer;
const checkSearch = ref("");
let checkSearchTimer;

const checkPagination = reactive({
  page: 1,
  rowsPerPage: TABLE_BATCH_SIZE,
  rowsNumber: 0,
  count: 0,
});
const checkActions = reactive({});
const hostExpanded = reactive({});
const checkExpanded = reactive({});

const checkSummaries = ref([]);
const loadingChecks = ref(false);
const checkDetailsState = reactive({});
const defaultCheckDetailsState = {
  page: 1,
  rowsPerPage: TABLE_BATCH_SIZE,
  rowsNumber: 0,
  count: 0,
  items: [],
  loading: false,
};
const hostDateModes = reactive({});
const checkDateModes = reactive({});

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

const checkGroups = computed(() => checkSummaries.value);

onMounted(() => {
  loadHosts();
  loadCheckSummaries();
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
    const pattern = buildSearchPattern(hostSearch.value);
    const data = await fetchHosts({ count: perPage, offset, hostNameSearch: pattern });
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

function handleHostSearchInput() {
  if (hostSearchTimer) {
    clearTimeout(hostSearchTimer);
  }
  hostSearchTimer = setTimeout(() => {
    hostPagination.page = 1;
    loadHosts();
  }, 200);
}

function handleCheckSearchInput() {
  if (checkSearchTimer) {
    clearTimeout(checkSearchTimer);
  }
  checkSearchTimer = setTimeout(() => {
    checkPagination.page = 1;
    loadCheckSummaries();
  }, 200);
}

async function loadCheckSummaries(nextPage, options = {}) {
  const refreshOpen = options.refreshOpen === true;
  if (typeof nextPage === "number") {
    checkPagination.page = nextPage;
  }
  const perPage = checkPagination.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, checkPagination.page);
  const offset = (page - 1) * perPage;
  loadingChecks.value = true;
  try {
    const pattern = buildSearchPattern(checkSearch.value);
    const data = await fetchCheckSummaries({
      count: perPage,
      offset,
      checkNameSearch: pattern,
    });
    checkSummaries.value = data.checks || [];
    checkPagination.count = data.count ?? checkSummaries.value.length;
    checkPagination.rowsNumber = data.total ?? checkPagination.count;
    const maxPage = checkMaxPage.value;
    if (page > maxPage && maxPage > 0) {
      checkPagination.page = maxPage;
      if (maxPage !== page) {
        await loadCheckSummaries(maxPage, options);
        return;
      }
    }
    if (refreshOpen) {
      const expandedChecks = checkSummaries.value
        .filter((check) => checkExpanded[check.CheckName])
        .map((check) => check.CheckName);
      await Promise.all(
        expandedChecks.map((checkName) => loadCheckDetails(checkName, getCheckDetailsState(checkName).page))
      );
    }
  } catch (err) {
    console.error("load checks", err);
  } finally {
    loadingChecks.value = false;
  }
}

function refreshChecks() {
  loadCheckSummaries(undefined, { refreshOpen: true });
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
    const data = await fetchCheckDetails({
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

function createCheckDetailsState() {
  return reactive({
    page: 1,
    rowsPerPage: TABLE_BATCH_SIZE,
    rowsNumber: 0,
    count: 0,
    items: [],
    loading: false,
  });
}

function resolveCheckDetailsState(checkName) {
  if (!checkName) {
    return createCheckDetailsState();
  }
  if (!checkDetailsState[checkName]) {
    checkDetailsState[checkName] = createCheckDetailsState();
  }
  return checkDetailsState[checkName];
}

function getCheckDetailsState(checkName) {
  if (!checkName) {
    return defaultCheckDetailsState;
  }
  return checkDetailsState[checkName] || defaultCheckDetailsState;
}

async function loadCheckDetails(checkName, nextPage) {
  if (!checkName) {
    return;
  }
  const state = resolveCheckDetailsState(checkName);
  if (typeof nextPage === "number") {
    state.page = nextPage;
  }
  const perPage = state.rowsPerPage || TABLE_BATCH_SIZE;
  const page = Math.max(1, state.page);
  const offset = (page - 1) * perPage;
  state.loading = true;
  try {
    const data = await fetchCheckDetails({
      checkName,
      count: perPage,
      offset,
    });
    state.items = data.items || [];
    state.count = data.count ?? state.items.length;
    state.rowsNumber = data.total ?? state.count;
    const maxPage = checkDetailsMaxPage(checkName);
    if (page > maxPage && maxPage > 0) {
      state.page = maxPage;
      if (maxPage !== page) {
        await loadCheckDetails(checkName, maxPage);
      }
    }
  } catch (err) {
    console.error("load check details", err);
  } finally {
    state.loading = false;
  }
}

function checkDetailsMaxPage(checkName) {
  const state = getCheckDetailsState(checkName);
  const total = state.rowsNumber || 0;
  const perPage = state.rowsPerPage || TABLE_BATCH_SIZE;
  if (total === 0) {
    return 1;
  }
  return Math.ceil(total / perPage);
}

function changeCheckDetailsPage(checkName, nextPage) {
  loadCheckDetails(checkName, nextPage);
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
    Object.keys(hostDateModes).forEach((key) => {
      const [hostname] = key.split("::");
      if (!allowed.has(hostname)) {
        delete hostDateModes[key];
      }
    });
  },
  { immediate: true }
);

watch(
  checkSummaries,
  (groups) => {
    const allowed = new Set(groups.map((group) => group.CheckName));
    Object.keys(checkExpanded).forEach((key) => {
      if (!allowed.has(key)) {
        delete checkExpanded[key];
      }
    });
    Object.keys(checkDetailsState).forEach((key) => {
      if (!allowed.has(key)) {
        delete checkDetailsState[key];
      }
    });
    Object.keys(checkDateModes).forEach((key) => {
      const [checkName] = key.split("::");
      if (!allowed.has(checkName)) {
        delete checkDateModes[key];
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

function hostKey(row) {
  return `${row.Hostname || ""}::${row.CheckName || ""}`;
}

function checkKey(row) {
  return `${row.CheckName || ""}::${row.Hostname || ""}`;
}

function toggleHostDateMode(key) {
  hostDateModes[key] = !hostDateModes[key];
}

function toggleCheckDateMode(key) {
  checkDateModes[key] = !checkDateModes[key];
}

function formattedDate(value, absoluteMode) {
  if (absoluteMode) {
    return formatAbsolute(value);
  }
  return formatRelative(value);
}

function buildSearchPattern(value) {
  const query = typeof value === "string" ? value.trim() : "";
  if (!query) {
    return "";
  }
  try {
    // Valid regex stays untouched so advanced users can provide anchors, groups, etc.
    // eslint-disable-next-line no-new
    new RegExp(query);
    return query;
  } catch (err) {
    const escaped = query.replace(/[.+?^${}()|[\]\\]/g, "\\$&").replace(/-/g, "\\-");
    return escaped.replace(/\*/g, ".*");
  }
}

function formatRelative(value) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const now = Date.now();
  const diffMs = now - date.getTime();
  if (diffMs < 0) {
    return "just now";
  }
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 1) {
    return "just now";
  }
  return formatDuration(seconds) + " ago";
}

function formatAbsolute(value) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatDuration(totalSeconds) {
  const units = [
    { label: "d", value: 86400 },
    { label: "h", value: 3600 },
    { label: "m", value: 60 },
    { label: "s", value: 1 },
  ];
  let remaining = totalSeconds;
  const parts = [];
  for (const unit of units) {
    if (remaining >= unit.value || (unit.label === "s" && parts.length === 0)) {
      const count = Math.floor(remaining / unit.value);
      if (count > 0 || unit.label === "s") {
        parts.push(`${count}${unit.label}`);
      }
      remaining -= count * unit.value;
    }
  }
  return parts.join("");
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
    await Promise.all([loadHosts(hostPagination.page), loadCheckSummaries(checkPagination.page)]);
    const hostVisible = hosts.value.some((host) => host.Hostname === row.Hostname);
  if (hostVisible) {
    await loadHostChecks(row.Hostname, getHostChecksState(row.Hostname).page);
  }
    const checkVisible = checkSummaries.value.some((check) => check.CheckName === row.CheckName);
    if (checkVisible) {
      await loadCheckDetails(row.CheckName, getCheckDetailsState(row.CheckName).page);
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
