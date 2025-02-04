import { useEffect, useState } from "react";
import { Button } from "@heroui/react";

import { scrollToTop } from "@/utils/window";

export function ScrollToTop() {
  const [show, setShow] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setShow(window.scrollY > 300);
    };

    window.addEventListener("scroll", handleScroll);

    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return show ? (
    <Button
      isIconOnly
      className="fixed bottom-2 right-4 z-50"
      onPress={scrollToTop}
    >
      ↑
    </Button>
  ) : null;
}
