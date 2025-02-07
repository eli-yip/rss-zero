import { Route, Routes } from "react-router-dom";

import AboutPage from "@/pages/about";
import ArchivePage from "@/pages/archive";
import IndexPage from "@/pages/index";
import RandomPage from "@/pages/random";
import BlogPage from "@/pages/statistics";

function App() {
  return (
    <Routes>
      <Route element={<IndexPage />} path="/" />
      <Route element={<RandomPage />} path="/random" />
      <Route element={<ArchivePage />} path="/archive" />
      <Route element={<BlogPage />} path="/statistics" />
      <Route element={<AboutPage />} path="/about" />
    </Routes>
  );
}

export default App;
