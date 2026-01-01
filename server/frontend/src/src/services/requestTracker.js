import { ref } from "vue";

const count = ref(0);

export const activeRequests = count;

export function incrementRequests() {
  count.value += 1;
}

export function decrementRequests() {
  if (count.value > 0) {
    count.value -= 1;
  }
}
