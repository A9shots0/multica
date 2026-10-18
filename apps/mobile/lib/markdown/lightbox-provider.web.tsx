import { createContext, use, useMemo, useState, type ReactNode } from "react";
import { Image, Modal, Pressable, View } from "react-native";
import { Text } from "@/components/ui/text";

interface LightboxApi {
  open: (uri: string, sequence?: readonly string[]) => void;
}
const LightboxContext = createContext<LightboxApi>({ open: () => undefined });
export function useLightbox(): LightboxApi { return use(LightboxContext); }

export function LightboxProvider({ children }: { children: ReactNode }) {
  const [images, setImages] = useState<string[]>([]);
  const [index, setIndex] = useState(0);
  const api = useMemo<LightboxApi>(() => ({
    open: (uri, sequence) => {
      const candidates = sequence?.length ? [...sequence] : [uri];
      const found = candidates.indexOf(uri);
      setImages(found >= 0 ? candidates : [uri]);
      setIndex(found >= 0 ? found : 0);
    },
  }), []);
  const close = () => setImages([]);
  return <LightboxContext.Provider value={api}>
    {children}
    <Modal visible={images.length > 0} transparent animationType="fade" onRequestClose={close}>
      <View className="flex-1 items-center justify-center bg-black/95 px-4">
        <Pressable className="absolute right-5 top-5 z-10 rounded-full bg-white/15 px-4 py-2" onPress={close} accessibilityRole="button" accessibilityLabel="Close image">
          <Text className="text-white">Close</Text>
        </Pressable>
        {images[index] ? <Image source={{ uri: images[index] }} resizeMode="contain" className="h-[80%] w-full" /> : null}
        {images.length > 1 ? <View className="mt-4 flex-row items-center gap-4">
          <Pressable disabled={index === 0} onPress={() => setIndex((value) => Math.max(0, value - 1))}><Text className={index === 0 ? "text-white/30" : "text-white"}>Previous</Text></Pressable>
          <Text className="text-white">{index + 1} / {images.length}</Text>
          <Pressable disabled={index === images.length - 1} onPress={() => setIndex((value) => Math.min(images.length - 1, value + 1))}><Text className={index === images.length - 1 ? "text-white/30" : "text-white"}>Next</Text></Pressable>
        </View> : null}
      </View>
    </Modal>
  </LightboxContext.Provider>;
}
