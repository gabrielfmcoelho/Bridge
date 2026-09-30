import { redirect } from "next/navigation";

// There's no global trash anymore: each inventory module keeps its own
// (the "Lixeira" button in its toolbar). Old links land on Hosts.
export default function TrashRedirectPage() {
  redirect("/hosts");
}
