import { createApp } from "vue";
import { Quasar, Notify } from "quasar";
import router from "./router";
import App from "./App.vue";
import quasarLang from "quasar/lang/en-US";
import "@quasar/extras/material-icons/material-icons.css";
import "quasar/dist/quasar.css";
import "./style.css";

const app = createApp(App);

app.use(router);
app.use(Quasar, {
  plugins: {
    Notify,
  },
  lang: quasarLang,
});

app.mount("#app");
