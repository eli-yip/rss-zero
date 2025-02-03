export const apiUrl = (() => {
  if (import.meta.env.DEV) {
    return "http://localhost:8080/api/v1";
  }

  return import.meta.env.BASE_URL + "api/v1";
})();
