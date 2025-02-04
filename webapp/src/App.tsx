import { Route, Routes } from "react-router-dom";

import IndexPage from "@/pages/index";
import RandomPage from "@/pages/random";
import ArchivePage from "@/pages/archive";
import BlogPage from "@/pages/blog";
import AboutPage from "@/pages/about";

function App() {
  return (
    <Routes>
      <Route element={<IndexPage />} path="/" />
      <Route element={<RandomPage />} path="/random" />
      <Route element={<ArchivePage />} path="/archive" />
      <Route element={<BlogPage />} path="/blog" />
      <Route element={<AboutPage />} path="/about" />
    </Routes>
  );
}

export default App;
