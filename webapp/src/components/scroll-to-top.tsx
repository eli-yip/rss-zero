import { Button } from "@heroui/react";
import { useEffect, useState } from "react";
import { FaArrowUp } from "react-icons/fa";

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
    <div className="fixed right-4 bottom-4 z-50">
      <Button isIconOnly className="rounded-full" onPress={scrollToTop}>
        <FaArrowUp />
      </Button>
    </div>
  ) : null;
}
