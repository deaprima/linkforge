import { cn } from "cn";
import type { CSSProperties } from "react";

/*
 * Maskot rubah LinkForge (Figma: Components → Mascot/Hero, node 11:51).
 * Ilustrasi line-art yang tersusun dari banyak vektor; posisi setiap potongan mengikuti Figma persis.
 * Aset light & dark berbeda (warna stroke/fill di-bake di SVG), jadi dua set dirender dan ditukar via CSS.
 */

const lightAssets = import.meta.glob<string>("../../assets/mascot/hero/*.svg", { eager: true, query: "?url", import: "default" });
const darkAssets = import.meta.glob<string>("../../assets/mascot/hero-dark/*.svg", { eager: true, query: "?url", import: "default" });

type AssetSet = Record<string, string>;

function bySet(assets: AssetSet, dir: string) {
  return (name: string) => assets[`../../assets/mascot/${dir}/${name}.svg`];
}

const containerSize: CSSProperties = { containerType: "size" };

function Art({ a }: { a: (name: string) => string }) {
  return (
    <div className="absolute contents inset-[6.13%_-0.79%_4.52%_-5.14%]">
      <div className="absolute flex inset-[49.09%_-0.79%_7.16%_47.6%] items-center justify-center" style={containerSize}>
        <div className="flex-none h-[hypot(-78.7755cqw,55.3009cqh)] rotate-60 w-[hypot(21.2245cqw,44.6991cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-0.5%_-1.07%]">
              <img alt="" className="block max-w-none size-full" src={a("vector")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute contents inset-[49.09%_-0.79%_7.16%_47.6%]">
        <div
          className="absolute inset-[45.17%_-9.25%_26.46%_79.46%] mask-alpha mask-intersect mask-no-clip mask-no-repeat mask-position-[-197.55px_24.34px] mask-size-[329.808px_271.244px]"
          style={{ maskImage: `url("${a("group")}")` }}
        >
          <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("group1")} />
        </div>
      </div>
      <div className="absolute inset-[85.81%_56.77%_4.52%_24.52%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector1")} />
      </div>
      <div className="absolute inset-[85.81%_34.19%_4.52%_47.1%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector1")} />
      </div>
      <div className="absolute inset-[43.55%_28.23%_8.06%_18.55%]">
        <div className="absolute inset-[-0.5%_-0.45%]">
          <img alt="" className="block max-w-none size-full" src={a("vector2")} />
        </div>
      </div>
      <div className="absolute inset-[55.97%_37.9%_9.19%_28.23%]">
        <div className="absolute inset-[-0.69%_-0.71%]">
          <img alt="" className="block max-w-none size-full" src={a("vector3")} />
        </div>
      </div>
      <div className="absolute inset-[41.94%_65.32%_42.26%_30.97%]">
        <div className="absolute inset-[-2.55%_-10.87%]">
          <img alt="" className="block max-w-none size-full" src={a("vector4")} />
        </div>
      </div>
      <div className="absolute inset-[41.94%_40.65%_42.26%_55.65%]">
        <div className="absolute inset-[-2.55%_-10.87%]">
          <img alt="" className="block max-w-none size-full" src={a("vector5")} />
        </div>
      </div>
      <div className="absolute inset-[55.16%_35.16%_11.61%_25.48%]">
        <div className="absolute inset-[-0.73%_-0.61%]">
          <img alt="" className="block max-w-none size-full" src={a("vector6")} />
        </div>
      </div>
      <div className="absolute inset-[56.94%_37.1%_13.06%_27.42%]">
        <div className="absolute inset-[-0.81%_-0.74%_-0.8%_-0.74%]">
          <img alt="" className="block max-w-none size-full" src={a("vector7")} />
        </div>
      </div>
      <div className="absolute inset-[70.97%_48.39%_19.03%_38.71%]">
        <div className="absolute inset-[-2.02%_-1.56%]">
          <img alt="" className="block max-w-none size-full" src={a("vector8")} />
        </div>
      </div>
      <div className="absolute flex inset-[71.98%_50.85%_20.04%_41.17%] items-center justify-center" style={containerSize}>
        <div className="-rotate-45 flex-none h-[hypot(25.7143cqw,25.7143cqh)] w-[hypot(74.2857cqw,-74.2857cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-16.67%_-5.77%]">
              <img alt="" className="block max-w-none size-full" src={a("group2")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute flex inset-[21.15%_74.9%_38.16%_-5.14%] items-center justify-center" style={containerSize}>
        <div className="-rotate-18 flex-none h-[hypot(37.09cqw,84.8128cqh)] w-[hypot(62.91cqw,-15.1872cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-0.67%_-1.21%]">
              <img alt="" className="block max-w-none size-full" src={a("group3")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute flex inset-[51.59%_65.76%_22.56%_10.28%] items-center justify-center" style={containerSize}>
        <div className="flex-none h-[hypot(-59.6828cqw,70.8042cqh)] rotate-38 w-[hypot(40.3172cqw,29.1958cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-1.04%_-1.97%]">
              <img alt="" className="block max-w-none size-full" src={a("vector9")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute inset-[51.61%_79.68%_38.06%_10%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector10")} />
      </div>
      <div className="absolute flex inset-[51.59%_19.96%_22.56%_56.09%] items-center justify-center" style={containerSize}>
        <div className="-rotate-38 flex-none h-[hypot(59.6828cqw,70.8042cqh)] w-[hypot(40.3172cqw,-29.1958cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-1.04%_-1.97%]">
              <img alt="" className="block max-w-none size-full" src={a("vector11")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute flex inset-[36.2%_7.17%_40.07%_69.11%] items-center justify-center" style={containerSize}>
        <div className="-rotate-45 flex-none h-[hypot(25cqw,25cqh)] w-[hypot(75cqw,-75cqh)]">
          <div className="relative size-full">
            <div className="absolute inset-[-15.38%_-5.13%]">
              <img alt="" className="block max-w-none size-full" src={a("group4")} />
            </div>
          </div>
        </div>
      </div>
      <div className="absolute inset-[51.61%_19.35%_38.06%_70.32%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector10")} />
      </div>
      <div className="absolute inset-[6.13%_57.74%_70.16%_27.92%]">
        <div className="absolute inset-[-1.02%_-1.69%]">
          <img alt="" className="block max-w-none size-full" src={a("vector12")} />
        </div>
      </div>
      <div className="absolute inset-[11.61%_60.97%_73.87%_30.89%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector13")} />
      </div>
      <div className="absolute inset-[6.13%_37.59%_70.16%_48.06%]">
        <div className="absolute inset-[-1.02%_-1.69%]">
          <img alt="" className="block max-w-none size-full" src={a("vector14")} />
        </div>
      </div>
      <div className="absolute inset-[11.61%_40.57%_73.87%_51.29%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector15")} />
      </div>
      <div className="absolute inset-[17.42%_33.55%_46.45%_23.87%]">
        <div className="absolute inset-[-0.67%_-0.57%]">
          <img alt="" className="block max-w-none size-full" src={a("vector16")} />
        </div>
      </div>
      <div className="absolute inset-[35.84%_34.84%_45.81%_25.16%]">
        <div className="absolute inset-[-1.32%_-0.6%]">
          <img alt="" className="block max-w-none size-full" src={a("vector17")} />
        </div>
      </div>
      <div className="absolute inset-[24.19%_34.52%_70.97%_24.84%]">
        <div className="absolute inset-[-26.67%_-3.18%]">
          <img alt="" className="block max-w-none size-full" src={a("vector18")} />
        </div>
      </div>
      <div className="absolute inset-[19.35%_56.77%_70.97%_33.55%]">
        <div className="absolute inset-[-2.5%]">
          <img alt="" className="block max-w-none size-full" src={a("vector19")} />
        </div>
      </div>
      <div className="absolute inset-[20.81%_58.23%_72.42%_35%]">
        <div className="absolute inset-[-3.57%]">
          <img alt="" className="block max-w-none size-full" src={a("vector20")} />
        </div>
      </div>
      <div className="absolute inset-[19.35%_43.23%_70.97%_47.1%]">
        <div className="absolute inset-[-2.5%]">
          <img alt="" className="block max-w-none size-full" src={a("vector19")} />
        </div>
      </div>
      <div className="absolute inset-[20.81%_44.68%_72.42%_48.55%]">
        <div className="absolute inset-[-3.57%]">
          <img alt="" className="block max-w-none size-full" src={a("vector20")} />
        </div>
      </div>
      <div className="absolute inset-[23.23%_52.26%_75.16%_42.58%]">
        <div className="absolute inset-[-12.5%_-3.91%]">
          <img alt="" className="block max-w-none size-full" src={a("vector21")} />
        </div>
      </div>
      <div className="absolute inset-[32.9%_59.52%_60.32%_35.32%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector22")} />
      </div>
      <div className="absolute inset-[32.9%_45%_60.32%_49.84%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector22")} />
      </div>
      <div className="absolute inset-[33.87%_60.32%_64.19%_37.74%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector23")} />
      </div>
      <div className="absolute inset-[33.87%_45.81%_64.19%_52.26%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector23")} />
      </div>
      <div className="absolute inset-[40%_47.42%_49.68%_37.74%]">
        <div className="absolute inset-[-2.34%_-1.63%]">
          <img alt="" className="block max-w-none size-full" src={a("vector24")} />
        </div>
      </div>
      <div className="absolute inset-[40.65%_52.1%_55.48%_42.42%]">
        <img alt="" className="absolute block inset-0 max-w-none size-full" src={a("vector25")} />
      </div>
      <div className="absolute inset-[45.81%_52.58%_52.9%_42.9%]">
        <div className="absolute inset-[-28.13%_-8.04%_-28.12%_-8.04%]">
          <img alt="" className="block max-w-none size-full" src={a("vector26")} />
        </div>
      </div>
    </div>
  );
}

/** Ukuran diatur lewat className (mis. "size-[110px]"); ilustrasi menskala proporsional. */
export function MascotHero({ className }: { className?: string }) {
  return (
    <div className={cn("relative shrink-0 overflow-clip", className)} role="img" aria-label="LinkForge fox mascot">
      <div className="absolute inset-0 dark:hidden">
        <Art a={bySet(lightAssets, "hero")} />
      </div>
      <div className="absolute inset-0 hidden dark:block">
        <Art a={bySet(darkAssets, "hero-dark")} />
      </div>
    </div>
  );
}
